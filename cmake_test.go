package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
)

// cmakeSnippet is cut down from a CMakeLists.txt whose fix joined the set, if
// and endif lines into one paragraph and wrote prose into the code.
const cmakeSnippet = `set(SGL_KERNEL_CUDA_FLAGS
    "-DNDEBUG"
    "--expt-extended-lambda"

    # The following flag leads to the CMAKE_BUILD_PARALLEL_LEVEL breaking,
    # it triggers OOM with low memory host. Extract the threads number to
    # option named SGL_KERNEL_COMPILE_THREADS, default value 32.
    # "--threads=32"

    "-Xcudafe=--diag_suppress=177"   # variable was declared but never referenced
)

# Restrict gencodes to the GPUs actually deployed, e.g. -DSGL_KERNEL_CUDA_ARCHS=120a
# for RTX PRO 6000 Blackwell. Empty builds every arch this CUDA version supports.
set(SGL_KERNEL_CUDA_ARCHS "" CACHE STRING "GPU archs to build for (e.g. 120a); empty builds all")
set(SGL_KERNEL_BUILD_SM90_VARIANT ON)
set(SGL_KERNEL_BUILD_FLASHMLA ON)
set(SGL_KERNEL_ARCH_GENCODES "")
if (SGL_KERNEL_CUDA_ARCHS)
    foreach(arch IN LISTS SGL_KERNEL_CUDA_ARCHS)
        list(APPEND SGL_KERNEL_ARCH_GENCODES "-gencode=arch=compute_${arch},code=sm_${arch}")
    endforeach()
    list(FILTER SGL_KERNEL_CUDA_FLAGS EXCLUDE REGEX "^-gencode=")
    # sgl_kernel/load_utils.py loads the sm90 copy only on SM90; FA3 is Hopper-only.
    if (NOT SGL_KERNEL_CUDA_ARCHS MATCHES "(^|;)90a?(;|$)")
        set(SGL_KERNEL_BUILD_SM90_VARIANT OFF)
        set(SGL_KERNEL_ENABLE_FA3 OFF)
    endif()
    message(STATUS "sgl-kernel: building only for ${SGL_KERNEL_CUDA_ARCHS}")
endif()

# ===================== InfLLM-V2 FlashAttention backend ===================== #
# Standalone pybind extension infllm_ops, vendored from
# 3rdparty/infllmv2_cuda_impl. Kept as its own module so its flash symbols
# stay isolated from sgl-kernel's own flash attention. Mirrors the original
# setup.py: only hdim 64/128 bf16 forward instantiations are compiled (the
# vendored static_switch.h forces bf16 and dispatches headdim to {64, 128}
# only). Backward kernels are intentionally omitted because SGLang only uses
# these ops for inference.
set(INFLLM_FLASH_CUDA_FLAGS
    "-DNDEBUG"
    "-O3"
)
`

// codeLines answers every line of a CMake listfile that is not a comment
// alone, in order. A repair must leave each of them as written.
func codeLines(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			out = append(out, line)
		}
	}
	return out
}

func TestACMakeListfileIsSourceNotProse(t *testing.T) {
	for _, path := range []string{"CMakeLists.txt", "python/sglang/kernels/aot/CMakeLists.txt", "cmake/flashmla.cmake"} {
		for _, finding := range slopfix.CheckContent(path, cmakeSnippet) {
			assert.NotContains(t, []string{"wrap/hard-wrap", "wrap/long-block"}, finding.ID, "%s: a prose rule read CMake code at line %d", path, finding.Line)
			assert.False(t, strings.HasPrefix(finding.ID, "ste/"), "%s: a prose rule read CMake code at line %d: %s", path, finding.Line, finding.ID)
		}
		repair := slopfix.Fix(slopfix.Request{Path: path, Content: cmakeSnippet})
		assert.Equal(t, codeLines(cmakeSnippet), codeLines(repair.Text), "%s: a repair changed a code line", path)
		assert.Empty(t, slopfix.CheckContent(path, repair.Text), "%s: fix left a finding check reports", path)
	}
}

// A '#' inside a bracket argument or a quoted argument is data, and a bracket
// comment is a comment no line rule may cut a piece of.
func TestCMakeBracketsAndQuotesAreNotLineComments(t *testing.T) {
	src := "#[[ A bracket comment that names 3 widgets\n" +
		"# and runs over a second line ]]\n" +
		"set(NOTE [=[\n" +
		"# 3 widgets live in this bracket argument\n" +
		"]=])\n" +
		"set(QUOTED \"\n" +
		"# 4 widgets live in this quoted argument\n" +
		"\")\n"
	assert.Empty(t, slopfix.CheckContent("CMakeLists.txt", src))
	assert.Equal(t, src, slopfix.Fix(slopfix.Request{Path: "CMakeLists.txt", Content: src}).Text)
}

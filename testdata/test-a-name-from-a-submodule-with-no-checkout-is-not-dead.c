#include "shim_internal.h"

// macOS supports ARB_debug_output in some drivers. We emulate the KHR_debug interface.
static int g_debug = 0;

// Requires GL_ARB_vertex_attrib_binding, stub.
static int g_binding = 0;

package bashclean

import (
 "regexp"
 "strings"
 "mvdan.cc/sh/v3/syntax"
)

func hasFileRead(f *syntax.File) bool { return hasStatementCall(f, func(c *syntax.CallExpr) bool { e,ok:=effectiveCommand(c); if !ok{return false}; if e.name!="cat"&&e.name!="head"&&e.name!="tail"&&e.name!="sed"{return false}; for _,w:=range c.Args[e.index+1:]{s,st:=literal(w); if st&&!strings.HasPrefix(s,"-")&&!strings.HasPrefix(s,"/"){return true}; if st&&strings.HasPrefix(s,"/")&&!strings.HasPrefix(s,"/proc/")&&!strings.HasPrefix(s,"/sys/")&&!strings.HasPrefix(s,"/dev/"){return true} }; return false }) }
func hasGitRM(f *syntax.File) bool { return hasStatementCall(f,func(c *syntax.CallExpr)bool{e,ok:=effectiveCommand(c);if !ok||e.name!="git"{return false}; for _,w:=range c.Args[e.index+1:]{s,st:=literal(w);if st&&s=="--cached"{return false};if st&&!strings.HasPrefix(s,"-"){return s=="rm"}};return false}) }
func hasBadTruncate(f *syntax.File)bool{return false}
func hasBadRM(f *syntax.File)bool{return hasStatementCall(f,func(c *syntax.CallExpr)bool{e,ok:=effectiveCommand(c);if !ok||e.name!="rm"{return false};for _,w:=range c.Args[e.index+1:]{s,st:=literal(w);if st&&strings.HasPrefix(s,"-")&&s!="--"&&!regexp.MustCompile(`^--(recursive|force|verbose|interactive)$|^-[rRfvIi]+$`).MatchString(s){return true}};return false})}

func trailing(f *syntax.File, fn func(*syntax.Stmt)){if len(f.Stmts)>0{fn(f.Stmts[len(f.Stmts)-1])}}
func stripStages(s *syntax.Stmt,names map[string]bool){b,ok:=s.Cmd.(*syntax.BinaryCmd);if !ok{return}; if b.Op==syntax.Pipe{if c,ok:=b.Y.Cmd.(*syntax.CallExpr);ok&&len(c.Args)>0&&names[cmd(c)]{s.Cmd=b.X.Cmd}}}
func stripOrTrue(s *syntax.Stmt){b,ok:=s.Cmd.(*syntax.BinaryCmd);if ok&&b.Op==syntax.OrStmt&&cmdCall(b.Y.Cmd)=="true"{s.Cmd=b.X.Cmd}}
func cmdCall(c syntax.Command)string{if x,ok:=c.(*syntax.CallExpr);ok{return cmd(x)};return ""}
func stripMerge(s *syntax.Stmt){if len(s.Redirs)>0{r:=s.Redirs[len(s.Redirs)-1];if r.Op==syntax.DplOut&&r.N!=nil&&r.N.Value=="2"&&isWord(r.Word,"1"){s.Redirs=s.Redirs[:len(s.Redirs)-1]}}}
func tee(s *syntax.Stmt){if len(s.Redirs)!=1{return};r:=s.Redirs[0];if r.Op!=syntax.RdrOut&&r.Op!=syntax.AppOut{return}; if strings.HasPrefix(litOf(r.Word),"/dev/"){return}; c,ok:=s.Cmd.(*syntax.CallExpr);if !ok{return};s.Cmd=&syntax.BinaryCmd{Op:syntax.Pipe,X:&syntax.Stmt{Cmd:c},Y:&syntax.Stmt{Cmd:&syntax.CallExpr{Args:[]*syntax.Word{word("tee")}}}};if r.Op==syntax.AppOut{s.Cmd.(*syntax.BinaryCmd).Y.Cmd.(*syntax.CallExpr).Args=append(s.Cmd.(*syntax.BinaryCmd).Y.Cmd.(*syntax.CallExpr).Args,word("-a"))};s.Cmd.(*syntax.BinaryCmd).Y.Cmd.(*syntax.CallExpr).Args=append(s.Cmd.(*syntax.BinaryCmd).Y.Cmd.(*syntax.CallExpr).Args,r.Word);s.Redirs=nil}
func ensurePipefail(f *syntax.File){if len(f.Stmts)==0||cmdCall(f.Stmts[0].Cmd)=="set"{return};f.Stmts=append([]*syntax.Stmt{{Cmd:&syntax.CallExpr{Args:[]*syntax.Word{word("set"),word("-o"),word("pipefail")}}}},f.Stmts...)}
func narration(f *syntax.File){walkCalls(f,func(c *syntax.CallExpr){e,ok:=effectiveCommand(c);if !ok|| (e.name!="echo"&&e.name!="printf")||len(c.Args)<=e.index+1{return};if allStatic(c.Args[e.index+1:]){c.Args=[]*syntax.Word{word(":")}}})}

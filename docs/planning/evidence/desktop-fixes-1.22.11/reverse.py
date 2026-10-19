"""Contrapruebas; ejecutar sin otras ediciones/tests. Restaura siempre con cp."""
from pathlib import Path as P
import subprocess as S,tempfile,json
root=P.cwd();dest=P(__file__).resolve().parent;backup=P(tempfile.mkdtemp(prefix='mup-reverse-'))
files=['internal/tui/console_wrap.go','internal/tui/view.go','internal/tui/help_view.go','internal/tui/download_exec.go','internal/tui/get_finish.go','internal/tui/tui.go']
for name in files:
 p=backup/name;p.parent.mkdir(parents=True,exist_ok=True);S.run(['cp',str(root/name),str(p)],check=True)
def restore():
 for name in files:S.run(['cp',str(backup/name),str(root/name)],check=True)
def edit(name,a,b):
 p=root/name;s=p.read_text();assert a in s,(name,a);p.write_text(s.replace(a,b))
def old_help():
 p=root/'internal/tui/help_view.go';s=p.read_text();old=(dest/'help-baseline.txt').read_text();start=old.index('func (m *Model) helpView()')
 a=s.index('func (m *Model) helpView()');b=s.index('func (m *Model) helpCard')
 p.write_text(s[:a]+old[start:].strip()+'\n\n'+s[b:])
cases=[
 ('wrap','TestDesktopConsoleWrapKeepsTextAndStyle',lambda:edit('internal/tui/console_wrap.go','var out []string','return append([]string(nil), m.conLines...)\n var out []string')),
 ('minimum','TestDesktopMinimumNoticeKeepsDimensions',lambda:edit('internal/tui/view.go','return strings.Join(styledLines(i18n.Tf("tui.too_small", minWidth, minHeight), m.width, m.height, m.st.text), "\\n")','return m.st.text.Render(i18n.Tf("tui.too_small", minWidth, minHeight))')),
 ('help','TestDesktopHelpCardsAndResponsiveLayout',old_help),
 ('capture','TestDesktopDownloadExecRetainsSafeDiagnostics',lambda:edit('internal/tui/download_exec.go','func (t *downloadTail) lines() []string {','func (t *downloadTail) lines() []string {\n return nil')),
 ('display','TestDesktopDownloadExecRetainsSafeDiagnostics',lambda:[edit(f,'m.printDownloadOutput(msg.output)','_ = msg.output') for f in ['internal/tui/get_finish.go','internal/tui/tui.go']]),
 ('bound','TestDesktopDownloadTailBoundedAndConcurrent',lambda:edit('internal/tui/download_exec.go','n := len(p)','t.data = append(t.data, p...)\n return len(p), nil\n n := len(p)')),
]
results=[]
try:
 for name,test,mutate in cases:
  mutate()
  args=['go','test','./internal/tui','-run','^'+test+'$','-count=1']
  broken=S.run(args,capture_output=True,text=True);log=broken.stdout+broken.stderr
  (dest/f'reverse-{name}.log').write_text(log)
  assert broken.returncode!=0 and '--- FAIL: '+test in log and '[build failed]' not in log,(name,log)
  restore()
  fixed=S.run(args,capture_output=True,text=True);assert fixed.returncode==0,(name,fixed.stdout,fixed.stderr)
  results.append({'case':name,'test':test,'compiled_expected_fail':True,'restored_pass':True});print(name,'FAIL compilable → OK',flush=True)
finally:restore()
(dest/'reverse.json').write_text(json.dumps(results,indent=2))

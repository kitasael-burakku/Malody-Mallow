"""Neutralizaciones compilables, una por vez; restaura SIEMPRE con cp.
dbus-run-session -- python3 reverse.py. Necesita mpv; no toca el índice de Git.
"""
import pathlib as P, tempfile, subprocess as S, os, json, sys
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS')
root=P.Path.cwd();dest=P.Path(__file__).resolve().parent;backup=P.Path(tempfile.mkdtemp(prefix='mu9reverse-'))
checks=[('registro',
  'internal/queue/queue.go',
  'q.rememberRemoval(i)',
  'q.DiscardUndo()',
  './internal/queue',
  'TestUndoRemoveEntry',
  'remove no habilitó recuperación'),
 ('token',
  'internal/queue/undo.go',
  'r == nil || (token != "" && token != r.token)',
  'r == nil',
  './internal/queue',
  'TestUndoRemoveExpiry',
  'cliente viejo recuperó otra eliminación'),
 ('actual',
  'internal/queue/undo.go',
  'q.Index++',
  'q.Index += 0',
  './internal/queue',
  'TestUndoRemoveEntry',
  'undo cambió la actual'),
 ('fifo',
  'internal/queue/undo.go',
  'if r.priority >= 0 {',
  'if false && r.priority >= 0 {',
  './internal/queue',
  'TestUndoRemovePriorityAndEmpty',
  'FIFO no recuperado'),
 ('shuffle',
  'internal/queue/undo.go',
  'slot := r.slot',
  'slot := 0',
  './internal/queue',
  'TestUndoRemoveEntry',
  'no restauró lugar en shuffle'),
 ('caducidad',
  'internal/queue/queue.go',
  'if len(tracks) > 0 {\n\t\tq.DiscardUndo()',
  'if len(tracks) > 0 {\n\t\t// caducidad neutralizada',
  './internal/queue',
  'TestUndoRemoveExpiry',
  'acción caducada aceptada'),
 ('fin',
  'internal/daemon/daemon_playback.go',
  'd.q.DiscardUndo()\n\t\tif reason == "error"',
  '// caducidad neutralizada\n\t\tif reason == "error"',
  './internal/daemon',
  'TestUndoDaemonFinalEnd',
  'fin de cola conservó recuperación'),
 ('audio',
  'internal/daemon/daemon.go',
  'return okStatus(i18n.TL(lang, "d.undone"))',
  'if tr, ok := d.q.Current(); ok { _ = d.pl.Load(tr.Path) }; return okStatus(i18n.TL(lang, "d.undone"))',
  './internal/daemon',
  'TestUndoDaemonPlayback',
  'undo reinició audio o perdió estado'),
 ('tui',
  'internal/tui/tui.go',
  'Cmd: "undo", Value: m.status.UndoToken',
  'Cmd: "undo", Value: ""',
  './internal/tui',
  'TestUndoTUIObservedToken',
  'no protege eliminación observada'),
 ('cli',
  'cmd/maly/client.go',
  'case "undo":\n\t\tif len(args) != 0',
  'case "undo":\n\t\tif false && len(args) != 0',
  './cmd/maly',
  'TestUndoCLI',
  'undo ignoró argumentos')]
results=[];logs=[]
if len(sys.argv)>1:
 checks=[c for c in checks if c[0] in sys.argv[1:]]
 results=json.loads((dest/'reverse.json').read_text());logs=[(dest/'reverse.log').read_text()]
env=os.environ|{'CGO_ENABLED':'0'}
for name,file,old,new,pkg,test,expected in checks:
 p=root/file;b=backup/name;S.run(['cp',str(p),str(b)],check=True)
 try:
  source=p.read_text();assert old in source;p.write_text(source.replace(old,new,1))
  args=['go','test',pkg,'-run','^'+test+'$','-count=1'];r=S.run(args,env=env,capture_output=True,text=True);out=r.stdout+r.stderr
  logs.append(name+' — neutralizado\n'+out)
  assert r.returncode!=0 and '[build failed]' not in out and expected in out,(name,out)
 finally:S.run(['cp',str(b),str(p)],check=True)
 r=S.run(args,env=env,capture_output=True,text=True);logs.append(name+' — restaurado\n'+r.stdout+r.stderr);assert r.returncode==0,r.stdout+r.stderr
 results.append({'case':name,'neutralized':'failed for expected assertion, compiled','restored':'passed'})
 (dest/'reverse.log').write_text('\n'.join(logs));(dest/'reverse.json').write_text(json.dumps(results,indent=2,ensure_ascii=False));print(name,'OK',flush=True)

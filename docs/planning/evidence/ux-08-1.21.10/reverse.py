"""Neutralizaciones compilables, una por vez; restaura SIEMPRE con cp.
dbus-run-session -- python3 reverse.py. Necesita mpv; no toca el índice de Git.
"""
import pathlib as P, tempfile, subprocess as S, os, json, sys
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS')
root=P.Path.cwd();dest=P.Path(__file__).resolve().parent;backup=P.Path(tempfile.mkdtemp(prefix='mu8reverse-'))
checks=[('prioridad', 'internal/queue/queue.go', 'if len(q.pending) > 0 {\n\t\treturn q.pending[0], true', 'if false && len(q.pending) > 0 {\n\t\treturn q.pending[0], true', './internal/queue', 'TestPlayNextFIFO', 'promesa FIFO'), ('consumo_fifo', 'internal/queue/queue.go', 'return q.JumpTo(q.pending[0])', 'return q.JumpTo(q.pending[len(q.pending)-1])', './internal/queue', 'TestPlayNextFIFO', 'no consumió la promesa'), ('move', 'internal/queue/queue.go', 'for k, e := range q.pending {', 'for k, e := range []int(nil) {', './internal/queue', 'TestPlayNextRemoveCurrentAndMovedPending', 'move perdió prioridad'), ('remove', 'internal/queue/queue.go', 'q.consumePending(i)\n\tfor k, v := range q.pending', '// consumo neutralizado\n\tfor k, v := range q.pending', './internal/queue', 'TestPlayNextMutations', 'remove dejó una referencia pendiente'), ('replace', 'internal/queue/queue.go', 'q.pending = nil', '// conservar pendientes', './internal/queue', 'TestPlayNextMutations', 'replace conservó prioridad'), ('restore', 'internal/daemon/session.go', 'd.q.RestorePending(pending)', '_ = pending', './internal/daemon', 'TestPlayNextSessionRemap', 'sesión perdió estado'), ('promesa_anterior', 'internal/daemon/daemon_playback.go', '} else if !d.stopped && d.q.PendingCount() > 0 {', '} else if false && !d.stopped && d.q.PendingCount() > 0 {', './internal/daemon', 'TestPlayNextLatePromise', 'reparación de promesa previa'), ('daemon', 'internal/daemon/daemon.go', 'd.q.AddNext(resolved...)', 'd.q.Add(resolved...)', './internal/daemon', 'TestPlayNextDaemonPausedAndFIFO', 'cola vacía arrancó o perdió pendientes'), ('cli', 'cmd/maly/client.go', 'case "add", "playnext":', 'case "add":', './cmd/maly', 'TestPlayNextCLI', 'gramática playnext incorrecta'), ('tui_ruta', 'internal/tui/songs.go', 'Cmd: "playnext", Paths: []string{it.value}', 'Cmd: "playnext", Paths: []string{it.label}', './internal/tui', 'TestPlayNextSurfaces', 'ruta seleccionada incorrecta'), ('marcador', 'internal/ipc/display.go', 's.NextPosition == index+1', 's.NextPosition == index+2', './internal/tui', 'TestPlayNextMarkerAndHints', 'no marca la siguiente')]
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

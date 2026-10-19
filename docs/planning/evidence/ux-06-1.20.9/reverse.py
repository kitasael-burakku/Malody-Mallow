"""Neutralizaciones compilables, una por vez; restaura SIEMPRE con cp.
python3 reverse.py. No necesita mpv ni toca el índice de Git.
"""
import pathlib as P, tempfile, subprocess as S, os, json, sys
root=P.Path.cwd();dest=P.Path(__file__).resolve().parent;backup=P.Path(tempfile.mkdtemp(prefix='mu6reverse-'))
checks=[
 ('atomicidad','internal/library/playlist.go','defer tx.Rollback()','defer tx.Commit()','./internal/library','TestSavePlaylistAtomic','escritura parcial'),
 ('orden','internal/library/playlist.go','id, tid, pos+1','id, tid, len(paths)-pos','./internal/library','TestSavePlaylistAtomic','quería'),
 ('instantanea','internal/playlist/save.go','paths[i] = track.Path','paths[i] = resp.Queue[0].Path\n _ = track','./internal/playlist','TestSaveQueueSnapshot','foto mezclada'),
 ('atajo','internal/tui/tui.go','m.is("save_queue", msg)','m.is("__disabled_save_queue", msg)','./internal/tui','TestSaveQueueShortcut','no preparó el nombre'),
 ('consola','internal/tui/console_playlist.go','case "save":','case "__disabled_save":','./internal/tui','TestConsoleSaveQueueUsesIPC','save no está conectado'),
 ('cli','cmd/maly/playlist.go','sub == "save"','sub == "__disabled_save"','./cmd/maly','TestPlaylistSaveGrammar','unknown playlist subcommand'),
]
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

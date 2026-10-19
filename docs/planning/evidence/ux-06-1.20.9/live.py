"""dbus-run-session -- python3 live.py /tmp/maly-ux06-after
XDG propio, mpv ao=null, tmux privado. Capturas de buffer, no de escritorio.
"""
import os, pathlib as P, subprocess as S, tempfile, time, json, socket, sys, shlex, sqlite3
binary=str(P.Path(sys.argv[1]).resolve());dest=P.Path(__file__).resolve().parent
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS')
root=P.Path(tempfile.mkdtemp(prefix='mu6-'));results=[]
for lang,profile,color in [('es','default',True),('en','vim',False),('es','override',False)]:
 r=root/profile
 for d in ['cfg/maly','cfg/mpv','data','run','music']:(r/d).mkdir(parents=True)
 os.chmod(r/'run',0o700)
 cfg=f'language="{lang}"\nupdate_check=false\nscan_durations=false\nmusic_dir="{r}/music"\ncontrols="{"vim" if profile=="vim" else "default"}"\n[visualizer]\nenabled=false\n[theme]\nbanner="off"\n'
 if profile=='override':cfg+='[keys]\nsave_queue="ctrl+y"\n'
 (r/'cfg/maly/config.toml').write_text(cfg);(r/'cfg/mpv/mpv.conf').write_text('ao=null\n')
 env=os.environ.copy();env.update(XDG_CONFIG_HOME=str(r/'cfg'),XDG_DATA_HOME=str(r/'data'),XDG_RUNTIME_DIR=str(r/'run'),XDG_CACHE_HOME=str(r/'cache'),TERM='xterm-256color');env.pop('NO_COLOR',None)
 if not color:env['NO_COLOR']='1'
 paths=[]
 for i,title in enumerate(['Otra dimensión','Cancio\u0301n 東京 — título largo de prueba']):
  path=r/f'music/{i}.flac';paths.append(str(path))
  S.run(['ffmpeg','-v','error','-f','lavfi','-i','anullsrc=r=8000:cl=mono','-t','120','-c:a','flac','-metadata',f'title={title}','-metadata','artist=Áurea','-y',str(path)],check=True)
 (r/'music/0.lrc').write_text('[00:00.00]Letra local\n[00:30.00]Otra línea\n')
 def cli(*args,code=0):
  p=S.run([binary,*args],env=env,capture_output=True,text=True);assert p.returncode==code,(args,p.stdout,p.stderr);return p.stdout+p.stderr
 target='ux:main';tmargs=['tmux','-S',str(r/'tmux')]
 def tm(*args):return S.check_output(tmargs+list(args),env=env,text=True,stderr=S.STDOUT)
 def key(*args):tm('send-keys','-t',target,*args)
 def type_text(text):
  S.run(tmargs+['load-buffer','-'],env=env,input=text,text=True,check=True)
  tm('paste-buffer','-d','-p','-t',target)
 def capture():return tm('capture-pane','-p','-t',target)
 def until(fn):
  deadline=time.monotonic()+12
  while time.monotonic()<deadline:
   try:
    value=fn()
    if value:return value
   except (FileNotFoundError,ConnectionRefusedError):pass
   time.sleep(.04)
  raise AssertionError('Estado esperado no llegó: '+capture())
 def wait(*want,absent=()):
  def check():
   out=capture();flat=' '.join(out.replace('│',' ').split())
   return out if all(x in flat for x in want) and all(x not in flat for x in absent) else None
  return until(check)
 def save(name):
  (dest/f'{profile}-{name}.txt').write_text(capture());(dest/f'{profile}-{name}.ansi').write_text(tm('capture-pane','-p','-e','-t',target))
 def ipc(cmd,**kw):
  with socket.socket(socket.AF_UNIX) as c:
   c.settimeout(12);c.connect(str(r/'run/maly/maly.sock'));c.sendall((json.dumps({'cmd':cmd,**kw})+'\n').encode());return json.loads(c.makefile().readline())
 def rows(name):
  with sqlite3.connect(r/'data/maly/library.db') as db:
   return [row[0] for row in db.execute('SELECT t.path FROM playlists p JOIN playlist_tracks pt ON p.id=pt.playlist_id JOIN tracks t ON t.id=pt.track_id WHERE p.name=? ORDER BY pt.pos',(name,))]
 def names():
  with sqlite3.connect(r/'data/maly/library.db') as db:return [x[0] for x in db.execute('SELECT name FROM playlists ORDER BY name')]
 def stable():
  q=ipc('queue');s=q['status'];return {'queue':[x['path'] for x in q.get('queue',[])],**{k:s.get(k) for k in ['queue_index','queue_gen','paused','position','shuffle','repeat','track']}}
 def query(text):
  key('End',*(['BSpace']*220));type_text(text)
  until(lambda:text in next((x for x in capture().splitlines() if '│❯ ' in x),''))
 daemon=None
 if profile=='override':daemon=S.Popen([binary,'daemon'],env=env,stdout=(r/'daemon.log').open('w'),stderr=S.STDOUT)
 tm('new-session','-d','-s','ux','-n','main','-x','140','-y','42',shlex.quote(binary))
 shortcut='C-y' if profile=='override' else 'C-s';label='ctrl+y' if profile=='override' else 'ctrl+s'
 try:
  until(lambda:(r/'run/maly/maly.sock').exists());cli('scan');wait('Áurea')
  # Error vacío en los dos clientes; cancelar el acceso no escribe nada.
  assert 'vacía' in cli('playlist','save','Vacía',code=1) if lang=='es' else 'empty' in cli('playlist','save','Vacía',code=1)
  key(shortcut);wait('playlist save');save('entry');key('Escape');assert not names()
  key(shortcut);type_text('Vacía');key('Enter');wait('vacía' if lang=='es' else 'empty');save('empty');assert not names();key('Escape')
  expected=[paths[1],paths[0],paths[1]]
  for path in expected:cli('add',path)
  cli('jump','2');until(lambda:ipc('status')['status'].get('track',{}).get('path')==paths[0]);cli('pause');until(lambda:ipc('status')['status']['paused']);cli('shuffle','on')
  # Esperar metadatos/posición asentados antes de comparar estado exacto.
  until(lambda:ipc('status')['status'].get('track',{}).get('duration',0)>=120)
  previous=stable()
  cli('playlist','save','Desde CLI');assert rows('Desde CLI')==expected
  assert stable()==previous,('CLI cambió reproducción',previous,stable())
  cli('playlist','save','Desde CLI',code=1);assert rows('Desde CLI')==expected
  # Cola filtrada a cero: Ctrl+S guarda TODO, también con shuffle.
  key('Tab','/');type_text('sin coincidencias');key('Enter');wait('sin coincidencias' if lang=='es' else 'no matches')
  for i,(w,h) in enumerate([(140,42),(100,28),(80,24),(40,12),(140,42)]):
   tm('resize-window','-t',target,'-x',str(w),'-y',str(h))
   key(shortcut);wait('playlist save');name=f'G{i}';type_text(name);key('Enter')
   until(lambda:rows(name)==expected);wait('Cola guardada' if lang=='es' else 'Queue saved');save(f'{w}x{h}-{i}-saved');key('Escape')
   assert stable()==previous,('TUI cambió reproducción',previous,stable())
   results.append({'profile':profile,'language':lang,'color':color,'size':[w,h],'filter_zero':True,'shuffle':True,'duplicates_order':True,'playback_unchanged':True})
  key('C-p');query('playlist save Desde CLI');key('Enter');wait('ya existe' if lang=='es' else 'already exists');save('occupied');assert rows('Desde CLI')==expected;key('Escape')
  # Falta una pista fuera del scan. El error deja las playlists intactas.
  foreign=r/'ajena.flac';foreign.write_bytes(P.Path(paths[0]).read_bytes());cli('add',str(foreign));before=names()
  out=cli('playlist','save','Incompleta',code=1);assert 'maly scan' in out and str(foreign) in out;assert names()==before
  tm('resize-window','-t',target,'-x','40','-y','12')
  key(shortcut);type_text('Incompleta');key('Enter');wait('maly scan');save('unindexed-40x12');assert names()==before;key('Escape')
  tm('resize-window','-t',target,'-x','140','-y','42')
  # Desde Ahora suena, el acceso prepara la entrada y Escape vuelve al panel.
  key('C-t');wait('Otra dimensión');key(shortcut);wait('playlist save');save('now-entry');key('Escape')
  # Un segundo cliente debe ver el refresh del guardado en el primero.
  cli('remove','4')
  if daemon:
   tm('new-window','-d','-t','ux','-n','observer',shlex.quote(binary));target='ux:observer';wait('Biblioteca');key('C-l');wait('Playlists');target='ux:main'
   key(shortcut);type_text('Compartida');key('Enter');until(lambda:rows('Compartida')==expected)
   target='ux:observer';wait('Compartida');save('second-client');key('C-c');target='ux:main';key('Escape')
   results.append({'two_clients_refresh':True})
  # También durante reproducción: misma pista, posición solo avanza naturalmente.
  cli('play');until(lambda:not ipc('status')['status']['paused']);s=ipc('status')['status'];cli('playlist','save','Sonando');after=ipc('status')['status'];assert rows('Sonando')==expected
  assert after['track']['path']==s['track']['path'] and not after['paused'] and after.get('position',0)>=s.get('position',0)
  cli('pause')
  # Ayuda usa el override real; el config viejo sin save_queue heredó el default.
  key('?');wait(label);save('help');key('Escape')
  results.append({'profile':profile,'cli_console_errors':True,'playing_save':True,'resolved_shortcut':label})
 finally:
  target='ux:main';key('C-c')
  if daemon:daemon.terminate();daemon.wait(timeout=12)
  until(lambda:not (r/'run/maly/maly.sock').exists())
 (dest/'matrix.json').write_text(json.dumps(results,indent=2,ensure_ascii=False));print(profile,'OK',flush=True)

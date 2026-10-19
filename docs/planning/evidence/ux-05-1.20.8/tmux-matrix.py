"""Prueba aislada: dbus-run-session -- python3 tmux-matrix.py BINARIO SALIDA [before].
Captura buffers de tmux; no son imágenes ni capturas del escritorio.
"""
import os, pathlib as P, subprocess as S, tempfile, time, json, socket, sys, shlex
binary=str(P.Path(sys.argv[1]).resolve());dest=P.Path(sys.argv[2]).resolve();dest.mkdir(parents=True,exist_ok=True)
before=len(sys.argv)>3 and sys.argv[3]=='before'
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS'), 'Hace falta un bus privado'
root=P.Path(tempfile.mkdtemp(prefix='mu5-'));results=[]
profiles=[('es','default',True)] if before else [('es','default',True),('en','vim',False),('es','override',False)]
for lang,profile,color in profiles:
 r=root/profile
 for d in ['cfg/maly','cfg/mpv','data','run','music']:(r/d).mkdir(parents=True)
 os.chmod(r/'run',0o700)
 cfg=f'language="{lang}"\nupdate_check=false\nscan_durations=false\nmusic_dir="{r}/music"\ncontrols="{"vim" if profile=="vim" else "default"}"\n[visualizer]\nenabled=false\n[theme]\nbanner="off"\n'
 if profile=='override':cfg+='[keys]\npalette="ctrl+y"\n'
 (r/'cfg/maly/config.toml').write_text(cfg);(r/'cfg/mpv/mpv.conf').write_text('ao=null\n')
 env=os.environ.copy();env.update(XDG_CONFIG_HOME=str(r/'cfg'),XDG_DATA_HOME=str(r/'data'),XDG_RUNTIME_DIR=str(r/'run'),XDG_CACHE_HOME=str(r/'cache'),TERM='xterm-256color');env.pop('NO_COLOR',None)
 if not color:env['NO_COLOR']='1'
 for i,title in enumerate(['Otra dimensión',"Don't Stop",'Una canción con un título muy largo — versión acústica','Cancio\u0301n 東京']):
  args=['ffmpeg','-v','error','-f','lavfi','-i','anullsrc=r=8000:cl=mono','-t','120','-c:a','flac','-metadata',f'title={title}','-metadata','artist=Áurea','-y',str(r/f'music/{i}.flac')]
  S.run(args,check=True)
 (r/'music/0.lrc').write_text('[00:00.00]Letra local\n[00:30.00]Otra línea\n')
 def cli(*args):return S.check_output([binary,*args],env=env,text=True,stderr=S.STDOUT)
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
 def wait_input(prefix):
  def check():
   out=capture()
   first=next((x for x in out.splitlines() if '│❯ ' in x),'')
   return out if first.split('│❯ ',1)[-1].startswith(prefix) else None
  return until(check)
 def save(name):
  prefix=f'{"before" if before else "after"}-{lang}-{profile}-{name}'
  (dest/f'{prefix}.txt').write_text(capture());(dest/f'{prefix}.ansi').write_text(tm('capture-pane','-p','-e','-t',target))
 def query(text):
  key('End',*(['BSpace']*220))
  if text:type_text(text)
  if text:wait_input(text)
 def ipc(cmd):
  with socket.socket(socket.AF_UNIX) as c:
   c.connect(str(r/'run/maly/maly.sock'));c.sendall((json.dumps({'cmd':cmd})+'\n').encode());return json.loads(c.makefile().readline())
 palette='C-y' if profile=='override' else 'C-p'
 daemon=None
 if profile=='override':daemon=S.Popen([binary,'daemon'],env=env,stdout=(r/'daemon.log').open('w'),stderr=S.STDOUT)
 tm('new-session','-d','-s','ux','-n','main','-x','80' if before else '140','-y','24' if before else '42',shlex.quote(binary))
 try:
  until(lambda:(r/'run/maly/maly.sock').exists());cli('scan');wait('Áurea')
  key(palette);wait('Paleta' if lang=='es' else 'Palette')
  if before:
   query('pl');key('Tab');out=wait('❯ pl');assert 'play [<' not in out;save('pl')
   query('help');key('Enter');wait('version');key('Up');wait('❯ help');save('history')
   results.append({'before':True,'no_suggestions':True,'help_and_history':True})
  else:
   for w,h in [(140,42),(100,28),(80,24),(40,12),(140,42)]:
    tm('resize-window','-t',target,'-x',str(w),'-y',str(h))
    query('pl');wait('play [<');key('Tab');wait('> play');save(f'{w}x{h}-name')
    key('Enter');wait_input('play ');query('elimina una playlist' if lang=='es' else 'delete a playlist');key('Tab');wait('> playlist delete');save(f'{w}x{h}-description')
    key('Enter');wait_input('playlist delete ');key('Enter');wait_input('comando…' if lang=='es' else 'command…');wait('uso:' if lang=='es' else 'usage:');save(f'{w}x{h}-incomplete')
    key('Escape');wait('Biblioteca' if lang=='es' else 'Library',absent=('Paleta','Palette'));key(palette)
    results.append({'language':lang,'profile':profile,'color':color,'size':[w,h],'selection_fill_invalid_escape':True})
   # La selección rellena; solo el Enter siguiente envía la acción real.
   query('add');key('Tab');key('Enter');type_text("'Otra dimensión' \"Don't Stop\"");key('Enter')
   expected=[str(r/'music/0.flac'),str(r/'music/1.flac')]
   until(lambda:[t['path'] for t in ipc('queue').get('queue',[])]==expected)
   cli('play',str(r/'music/1.flac'));until(lambda:ipc('status')['status'].get('track',{}).get('path')==expected[1])
   query('play');key('Tab');key('Enter');type_text("'Otra dimensión'");key('Enter')
   until(lambda:ipc('status')['status'].get('track',{}).get('path')==expected[0])
   query('pause');key('Enter');until(lambda:ipc('status')['status']['paused'])
   # Up/Down deben recuperar la línea y luego el borrador con comilla abierta.
   query("borrador 'abierto");key('Up');wait_input('pause');key('Down');wait_input("borrador 'abierto");save('history-draft')
   query('help');key('Enter');wait('get playlist');key('C-Home');wait('comandos disponibles' if lang=='es' else 'available commands');save('scroll-top')
   key('C-End');wait('get playlist');save('scroll-bottom')
   cli('playlist','create','Mi lista');cli('playlist','add','Mi lista','Otra dimensión')
   for response in ['escape','empty','no','yes']:
    query('playlist de');key('Tab');key('Enter');type_text("'Mi lista'");key('Enter');wait('s + enter' if lang=='es' else 'y + enter');save('confirm-'+response)
    assert 'Mi lista' in cli('playlist','list')
    key('Tab');wait('s + enter' if lang=='es' else 'y + enter')
    if response=='escape':key('Escape')
    elif response=='empty':key('Enter')
    else:type_text(response);key('Enter')
    if response=='yes':until(lambda:'Mi lista' not in cli('playlist','list'))
    else:
     wait('se mantiene' if lang=='es' else 'kept');assert 'Mi lista' in cli('playlist','list')
   results.append({'language':lang,'profile':profile,'queue_paths':expected,'played_path':expected[0],'paused':True,'history_draft_scroll':True,'delete_cancel_empty_no_yes':True,'service':bool(daemon)})
   # Dos clientes sobre el servicio privado: uno observa la playlist creada
   # desde la paleta del otro (el contrato de refresco sigue funcionando).
   if daemon:
    tm('new-window','-d','-t','ux','-n','observer',shlex.quote(binary));target='ux:observer';wait('Biblioteca');key('C-l');wait('Playlists');target='ux:main'
    query('playlist create');key('Tab');key('Enter');type_text("'Compartida'");key('Enter');until(lambda:'Compartida' in cli('playlist','list'))
    target='ux:observer';wait('Compartida');save('second-client');key('C-c');target='ux:main'
    results.append({'two_clients_playlist_refresh':True})
 finally:
  target='ux:main';key('C-c')
  if daemon:
   daemon.terminate();daemon.wait(timeout=12)
  until(lambda:not (r/'run/maly/maly.sock').exists())
 (dest/('matrix-before.json' if before else 'matrix.json')).write_text(json.dumps(results,indent=2,ensure_ascii=False))
 print(lang,profile,'OK',flush=True)

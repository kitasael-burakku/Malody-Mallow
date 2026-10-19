"""dbus-run-session -- python3 live.py /tmp/maly-ux08-after
TUI + CLI reales; XDG temporal, mpv ao=null, sin red ni servicio del usuario.
"""
import os,pathlib as P,subprocess as S,tempfile,time,json,socket,shlex,sys,wave,unicodedata
binary=str(P.Path(sys.argv[1]).resolve());dest=P.Path(__file__).resolve().parent
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS')
root=P.Path(tempfile.mkdtemp(prefix='mu8-'));results=[]
for lang,profile,color in [('es','default',True),('en','vim',False),('es','override',False)]:
 r=root/profile
 for d in ['cfg/maly','cfg/mpv','data','run','music']:(r/d).mkdir(parents=True)
 os.chmod(r/'run',0o700)
 cfg=f'language="{lang}"\nupdate_check=false\nscan_durations=false\nmusic_dir="{r}/music"\ncontrols="{"vim" if profile=="vim" else "default"}"\n[visualizer]\nenabled=false\n[theme]\nbanner="off"\n'
 if profile=='override':cfg+='[keys]\nplay_next="ctrl+y"\n'
 (r/'cfg/maly/config.toml').write_text(cfg);(r/'cfg/mpv/mpv.conf').write_text('ao=null\n')
 paths=[]
 for name in ['A actual','B Áurea 東京','C otra toma con título largo','D Café sin álbum']:
  p=r/'music'/(name+'.wav');paths.append(str(p))
  with wave.open(str(p),'wb') as w:w.setnchannels(1);w.setsampwidth(1);w.setframerate(8000);w.writeframes(bytes([128])*8000*120)
 env=os.environ|{'XDG_CONFIG_HOME':str(r/'cfg'),'XDG_DATA_HOME':str(r/'data'),'XDG_RUNTIME_DIR':str(r/'run'),'XDG_CACHE_HOME':str(r/'cache'),'TERM':'xterm-256color'};env.pop('NO_COLOR',None)
 if not color:env['NO_COLOR']='1'
 tmargs=['tmux','-S',str(r/'tmux')];target='ux:main'
 def tm(*args):return S.check_output(tmargs+list(args),env=env,text=True,stderr=S.STDOUT)
 def key(*args):tm('send-keys','-t',target,*args)
 def type_text(text):
  S.run(tmargs+['load-buffer','-'],env=env,input=text,text=True,check=True);tm('paste-buffer','-d','-p','-t',target)
 def capture(t=None):return tm('capture-pane','-p','-t',t or target)
 def until(fn):
  end=time.monotonic()+15
  while time.monotonic()<end:
   try:
    value=fn()
    if value:return value
   except (FileNotFoundError,ConnectionRefusedError):pass
   time.sleep(.05)
  raise AssertionError('Estado no llegó: '+capture())
 def wait(*words):return until(lambda:all(x in ' '.join(capture().replace('│',' ').split()) for x in words))
 def resized(w):
  prefix='╭─ '+(('Cola' if lang=='es' else 'Queue') if w<90 else ('Biblioteca' if lang=='es' else 'Library'))
  def ready():
   line=capture().splitlines()[0]
   width=sum(0 if unicodedata.combining(c) else 2 if unicodedata.east_asian_width(c) in ('W','F') else 1 for c in line)
   return line.startswith(prefix) and width==w
  until(ready)

 def save(name,t=None):
  (dest/f'{profile}-{name}.txt').write_text(capture(t));(dest/f'{profile}-{name}.ansi').write_text(tm('capture-pane','-p','-e','-t',t or target))
 def cli(*args):
  p=S.run([binary,*args],env=env,capture_output=True,text=True);assert p.returncode==0,(args,p.stdout,p.stderr);return p.stdout+p.stderr
 def ipc(cmd):
  with socket.socket(socket.AF_UNIX) as c:
   c.settimeout(12);c.connect(str(r/'run/maly/maly.sock'));c.sendall((json.dumps({'cmd':cmd})+'\n').encode());return json.loads(c.makefile().readline())
 def status():return ipc('status')['status']
 def pending():return status().get('next_count',0)
 def playerpath():
  with socket.socket(socket.AF_UNIX) as c:
   c.settimeout(3);c.connect(str(r/'run/maly/mpv.sock'));c.sendall(b'{"command":["get_property","path"],"request_id":87}\n')
   f=c.makefile()
   while True:
    x=json.loads(f.readline())
    if x.get('request_id')==87:return x.get('data')
 cli('scan')
 daemon=None
 if profile=='override':daemon=S.Popen([binary,'daemon'],env=env,stdout=(r/'daemon.log').open('w'),stderr=S.STDOUT)
 tm('new-session','-d','-s','ux','-n','main','-x','140','-y','42',binary)
 nextkey='C-y' if profile=='override' else 'C-n'
 try:
  until(lambda:(r/'run/maly/maly.sock').exists());wait('Biblioteca' if lang=='es' else 'Library')
  cli('play',paths[0]);cli('pause');cli('seek','3');until(lambda:status()['paused'] and status()['position']>=2.5)
  cli('shuffle','on');cli('repeat','one');before=status()
  cli('playnext',paths[1],paths[2]);until(lambda:pending()==2)
  after=status();assert after['track']['path']==paths[0] and after['paused'] and abs(after['position']-before['position'])<.2
  mark='[después]' if lang=='es' else '[next]';wait(mark);save('scheduled')
  tm('new-window','-d','-t','ux','-n','second',binary)
  until(lambda:mark in capture('ux:second'));save('second-client','ux:second');key('Tab')
  for w,h in [(140,42),(100,28),(80,24),(40,12),(140,42)]:
   tm('resize-window','-t',target,'-x',str(w),'-y',str(h));resized(w);wait(mark);save(f'{w}x{h}');assert pending()==2
   results.append({'profile':profile,'language':lang,'color':color,'size':[w,h],'paused_track_preserved':True})
  # Selector local: FIFO de selecciones, no búsqueda ambigua por título.
  key('C-o');type_text('f:C');wait('C otra');key(nextkey);until(lambda:pending()==3);save('songs-next');key('Escape')
  # Consola con comillas y ruta exacta.
  key('C-p');type_text('playnext '+json.dumps(paths[3],ensure_ascii=False));key('Enter');until(lambda:pending()==4);save('console-next');key('Escape')
  # Cola filtrada: copiar precisamente la entrada visible.
  key('/');type_text('D Cafe');key('Enter');wait(('Cola' if lang=='es' else 'Queue')+' (5) · 1/1','/D Cafe','ctrl+y' if profile=='override' else 'ctrl+n');save('filter-ready');key(nextkey);until(lambda:pending()==5);save('queue-filter-next');key('Escape')
  # Selector CLI independiente con el mismo override.
  tm('new-window','-d','-t','ux','-n','select',shlex.join([binary,'select']))
  target='ux:select';until(lambda:'enter' in capture());type_text('f:B');key(nextkey);until(lambda:pending()==6);save('select-next');key('Escape');target='ux:main'
  until(lambda:mark in capture('ux:second'));save('second-refreshed','ux:second')
  (dest/f'{profile}-queue-cli.txt').write_text(cli('queue'))
  # Recorrido efectivo: también preguntar a mpv, no solo a la copia de cola.
  for want in [paths[1],paths[2],paths[2],paths[3],paths[3],paths[1]]:
   cli('next');until(lambda:status().get('track',{}).get('path')==want and playerpath()==want)
  assert pending()==0 and status()['repeat']=='one';cli('pause')
  # Reponer prioridades y reiniciar el servicio completo, no solo la TUI.
  cli('playnext',paths[2],paths[3]);assert pending()==2
  tm('send-keys','-t','ux:second','C-c');key('C-c')
  if daemon:cli('kill');daemon.wait(timeout=10);daemon=None
  until(lambda:not (r/'run/maly/maly.sock').exists())
  saved=json.loads((r/'data/maly/session.json').read_text());assert len(saved['play_next'])==2
  daemon=S.Popen([binary,'daemon'],env=env,stdout=(r/'restart.log').open('w'),stderr=S.STDOUT)
  until(lambda:(r/'run/maly/maly.sock').exists() and ipc('ping').get('ok'))
  restored=until(lambda:status() if status()['paused'] and pending()==2 else None)
  assert restored['track']['path']==paths[1] and restored['shuffle']
  for want in paths[2:4]:cli('next');until(lambda:playerpath()==want)
  results.append({'profile':profile,'two_clients_refreshed':True,'six_exact_path_transitions':True,'session_restored_paused':True,'fifo_restored':True})
 finally:
  if daemon:daemon.terminate();daemon.wait(timeout=12)
  try:tm('kill-session','-t','ux')
  except S.CalledProcessError:pass
 (dest/'matrix.json').write_text(json.dumps(results,indent=2,ensure_ascii=False));print(profile,'OK',flush=True)

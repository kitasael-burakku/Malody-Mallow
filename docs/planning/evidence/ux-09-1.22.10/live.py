"""dbus-run-session -- python3 live.py /tmp/maly-ux09-after
TUI + CLI reales; XDG temporal, mpv ao=null, sin red ni servicio del usuario.
"""
import os,pathlib as P,subprocess as S,tempfile,time,json,socket,shlex,sys,wave,unicodedata
binary=str(P.Path(sys.argv[1]).resolve());dest=P.Path(__file__).resolve().parent
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS')
root=P.Path(tempfile.mkdtemp(prefix='mu9-'));results=[]
for lang,profile,color in [('es','default',True),('en','vim',False),('es','override',False)]:
 r=root/profile
 for d in ['cfg/maly','cfg/mpv','data','run','music']:(r/d).mkdir(parents=True)
 os.chmod(r/'run',0o700)
 cfg=f'language="{lang}"\nupdate_check=false\nscan_durations=false\nmusic_dir="{r}/music"\ncontrols="{"vim" if profile=="vim" else "default"}"\n[visualizer]\nenabled=false\n[theme]\nbanner="off"\n'
 if profile=='override':cfg+='[keys]\nundo_remove="ctrl+y"\n'
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
 def ipc(cmd,**args):
  with socket.socket(socket.AF_UNIX) as c:
   c.settimeout(12);c.connect(str(r/'run/maly/maly.sock'));c.sendall((json.dumps({'cmd':cmd}|args)+'\n').encode());return json.loads(c.makefile().readline())
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
 undokey='C-y' if profile=='override' else 'u'
 try:
  until(lambda:(r/'run/maly/maly.sock').exists());wait('Biblioteca' if lang=='es' else 'Library')
  cli('add',*paths);cli('pause');cli('seek','3');until(lambda:status()['paused'] and status()['position']>=2.5)
  cli('shuffle','on');cli('repeat','one');before=status()
  tm('new-window','-d','-t','ux','-n','second',binary)
  until(lambda:('Biblioteca' if lang=='es' else 'Library') in capture('ux:second'))
  key('Tab');cli('remove','3');token=until(lambda:status().get('undo_token'))
  mark='deshacer' if lang=='es' else 'undo';keylabel='ctrl+y' if profile=='override' else 'u'
  wait(mark);save('removed')
  until(lambda:mark in capture('ux:second'));save('second-available','ux:second')
  # Tras el flash, el pie conserva la acción disponible a todos los tamaños.
  for w,h in [(140,42),(100,28),(80,24),(40,12),(140,42)]:
   tm('resize-window','-t',target,'-x',str(w),'-y',str(h));resized(w)
   until(lambda:keylabel+' '+mark in capture());save(f'{w}x{h}')
   assert status()['undo_token']==token and status()['track']['path']==paths[0] and status()['paused']
   results.append({'profile':profile,'language':lang,'color':color,'size':[w,h],'undo_visible':True})
  # La otra TUI recupera la eliminación compartida sin reiniciar audio.
  tm('send-keys','-t','ux:second',undokey);until(lambda:status()['queue_len']==4 and not status().get('undo_token'))
  assert status()['track']['path']==paths[0] and abs(status()['position']-before['position'])<.2
  save('restored');save('second-restored','ux:second')
  # Cola filtrada: quitar exactamente D y deshacer desde la consola.
  key('/');type_text('D Cafe');key('Enter');wait(('Cola' if lang=='es' else 'Queue')+' (4) · 1/1','/D Cafe')
  key('x' if profile=='vim' else 'd');until(lambda:status()['queue_len']==3 and status().get('undo_token'));save('filtered-removed')
  assert all(t['path']!=paths[3] for t in ipc('queue')['queue'])
  key('Escape');key('C-p');type_text('undo');key('Enter');until(lambda:status()['queue_len']==4);save('console-restored');key('Escape')
  # Token anterior nunca recupera una eliminación nueva ni pisa su registro.
  cli('remove','4');old=status()['undo_token'];cli('remove','3');fresh=status()['undo_token']
  stale=ipc('undo',value=old);assert not stale['ok'] and status()['undo_token']==fresh
  (dest/f'{profile}-stale.json').write_text(json.dumps(stale,indent=2,ensure_ascii=False))
  cli('undo');assert [t['path'] for t in ipc('queue')['queue']]==paths[:3]
  # Recuperar la antigua actual conserva la que pasó a sonar, pausada.
  cli('remove','1');until(lambda:playerpath()==paths[1]);cli('seek','4');until(lambda:status()['position']>=3.5)
  current=status();cli('undo');assert status()['track']['path']==paths[1] and status()['paused'] and abs(status()['position']-current['position'])<.2
  until(lambda:playerpath()==paths[1]);save('old-current-restored')
  # Un cambio de cola caduca la acción y el error no revierte la nueva cola.
  cli('remove','3');cli('add',paths[3]);stale=ipc('undo');assert not stale['ok'] and [t['path'] for t in ipc('queue')['queue']]==[paths[0],paths[1],paths[3]]
  # El reinicio conserva contenido, nunca la posibilidad de deshacer.
  cli('remove','3');token=status()['undo_token'];tm('send-keys','-t','ux:second','C-c');key('C-c')
  if daemon:cli('kill');daemon.wait(timeout=10);daemon=None
  until(lambda:not (r/'run/maly/maly.sock').exists())
  daemon=S.Popen([binary,'daemon'],env=env,stdout=(r/'restart.log').open('w'),stderr=S.STDOUT)
  until(lambda:(r/'run/maly/maly.sock').exists() and ipc('ping').get('ok'))
  assert not status().get('undo_token') and not ipc('undo',value=token)['ok']
  results.append({'profile':profile,'shared_undo':True,'filtered_exact_path':True,'old_current_keeps_audio':True,'stale_rejected_without_overwrite':True,'restart_expires':True})
 finally:
  if daemon:daemon.terminate();daemon.wait(timeout=12)
  try:tm('kill-session','-t','ux')
  except S.CalledProcessError:pass
 (dest/'matrix.json').write_text(json.dumps(results,indent=2,ensure_ascii=False));print(profile,'OK',flush=True)

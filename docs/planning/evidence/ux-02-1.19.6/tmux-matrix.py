# Recorrido medido el 2026-10-18; adaptado para recibir binario y crear XDG nuevo.
import os,subprocess as S,pathlib as P,time,json,tempfile,sys,shlex
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS'), 'Ejecutar bajo dbus-run-session'
binary=str(P.Path(sys.argv[1]).resolve())
root=P.Path(tempfile.mkdtemp(prefix='maly-u2-'));results=[]
print(root,flush=True)
for lang in ['es','en']:
 for preset in ['default','vim','override']:
  for color in ['normal','no-color']:
   name=f'{lang}-{preset}-{color}';r=root/name
   for d in ['config/maly','config/mpv','data','run','music','caps']:(r/d).mkdir(parents=True,exist_ok=True)
   os.chmod(r/'run',0o700)
   cfg=f'language="{lang}"\ncontrols="{preset if preset!="override" else "default"}"\nupdate_check=false\nscan_durations=false\nmusic_dir="{r}/music"\n[theme]\nbanner="off"\n[visualizer]\nenabled=false\n'
   if preset=='override':cfg+='[keys]\nhelp="f1"\nquit="f2"\nfilter="f6"\nadd=" "\nswitch_panel="f8"\nplay_pause="ctrl+v"\npalette="ctrl+y"\n'
   (r/'config/maly/config.toml').write_text(cfg);(r/'config/mpv/mpv.conf').write_text('ao=null\n')
   e=os.environ.copy();e.update(XDG_CONFIG_HOME=str(r/'config'),XDG_DATA_HOME=str(r/'data'),XDG_RUNTIME_DIR=str(r/'run'),XDG_CACHE_HOME=str(r/'cache'),TERM='xterm-256color')
   if color=='normal':e.pop('NO_COLOR',None)
   else:e['NO_COLOR']='1'
   def cli(*args):return S.check_output([binary,*args],env=e,text=True,stderr=S.STDOUT)
   def tm(*args):return S.check_output(['tmux','-S',str(r/'tmux'),*args],env=e,text=True,stderr=S.STDOUT)
   def key(*a):tm('send-keys','-t','ux',*a)
   def cap():return tm('capture-pane','-p','-t','ux')
   def wait(want,absent=()):
    deadline=time.monotonic()+12
    while time.monotonic()<deadline:
     out=cap()
     if all(x in out for x in want) and all(x not in out for x in absent):return out
     time.sleep(.04)
    raise AssertionError((name,want,absent,out))
   def save(n):
    (r/'caps'/f'{n}.txt').write_text(cap());(r/'caps'/f'{n}.ansi').write_text(tm('capture-pane','-p','-e','-t','ux'))
   helpkey='F1' if preset=='override' else '?';panelkey='F8' if preset=='override' else 'Tab';filterkey='F6' if preset=='override' else '/';addkey='Space' if preset=='override' else 'a';pausekey='C-v' if preset=='override' else 'Space';conkey='C-y' if preset=='override' else 'C-p'
   helptext=('f1' if preset=='override' else '?')+(' ayuda' if lang=='es' else ' help');exittext=('f2' if preset=='override' else 'q')+(' salir' if lang=='es' else ' quit');closetext='esc cierra' if lang=='es' else 'esc closes'
   tm('new-session','-d','-s','ux','-x','140','-y','42',shlex.quote(binary))
   try:
    wait(['maly scan']);save('first-start')
    # Audio y carátula sintéticos; no se accede a música ni imágenes del usuario.
    ppm=r/'cover.ppm';ppm.write_bytes(b'P6\n8 8\n255\n'+bytes([40,160,160])*64)
    S.run(['ffmpeg','-v','error','-f','lavfi','-i','anullsrc=r=8000:cl=mono','-i',str(ppm),'-t','90','-map','0:a','-map','1:v','-c:a','flac','-c:v','png','-disposition:v','attached_pic','-metadata','title=Luz de octubre','-metadata','artist=Áurea','-metadata','album=Horizontes','-y',str(r/'music/luz.flac')],check=True)
    (r/'music/luz.lrc').write_text('[00:00.00]Luz de octubre\n[00:03.00]Un verso local\n')
    cli('scan');wait(['Áurea']);key(addkey);wait(['Luz de octubre']);key(pausekey);wait(['⏸'])
    # Dejar caducar el flash: el pie normal es la condición que interesa.
    wait([helptext]);(r/'cli-help.txt').write_text(cli('-h'))
    for w,h in [(140,42),(100,28),(80,24),(40,12),(140,42)]:
     tm('resize-window','-t','ux','-x',str(w),'-y',str(h));wait([helptext,exittext]);save(f'{w}x{h}-library')
     if w>=80:wait(['enter expande' if lang=='es' else 'enter expand',('espacio' if lang=='es' else 'space')+' agrega' if preset=='override' and lang=='es' else ('space add' if preset=='override' else ('a agrega' if lang=='es' else 'a add'))])
     key('Enter')
     if w>=80:wait(['enter pliega' if lang=='es' else 'enter collapse'])
     key('Enter')
     key(filterkey);key('-l','Luz');wait(['enter fija' if lang=='es' else 'enter accepts']);save(f'{w}x{h}-filter')
     key('Enter')
     if w>=80:wait(['enter reproduce' if lang=='es' else 'enter play'])
     key('Escape');key(panelkey);wait([helptext,exittext,'biblioteca' if lang=='es' else 'library']);save(f'{w}x{h}-queue')
     key(panelkey);key(helpkey);wait([closetext,'Reproducción' if lang=='es' else 'Playback']);save(f'{w}x{h}-help')
     for _ in range(30):key('PgDn')
     wait(['salir' if lang=='es' else 'quit']);save(f'{w}x{h}-help-end');key('Escape')
     key('C-o');wait([closetext,'Luz de octubre']);save(f'{w}x{h}-songs');key('Escape')
     key('C-l');wait([closetext,'ctrl+n']);save(f'{w}x{h}-playlists');key('Escape')
     key(conkey);wait([closetext]);save(f'{w}x{h}-console');key('Escape')
     key('C-t');wait([closetext,helptext]);save(f'{w}x{h}-now');key('Escape')
     results.append({'run':name,'size':f'{w}x{h}','ok':True})
    # Flash largo dentro de canciones: conserva cierre y no ensancha el frame.
    key('C-o');wait([closetext]);key('Tab');wait(['agreg' if lang=='es' else 'added']);save('songs-flash');key('Escape')
   finally:
    key('C-c')
    deadline=time.monotonic()+10
    while (r/'run/maly/maly.sock').exists() and time.monotonic()<deadline:time.sleep(.04)
    assert not (r/'run/maly/maly.sock').exists()
   (root/'matrix-results.json').write_text(json.dumps(results,indent=2));print(name,'OK',flush=True)

import os, subprocess as sp, pathlib as P, time, json, tempfile, argparse
parser=argparse.ArgumentParser(description='Aceptación aislada de UX-01; ejecutar dentro de dbus-run-session.')
parser.add_argument('binary', help='Binario compilado que se va a probar')
args=parser.parse_args()
binary=str(P.Path(args.binary).resolve())
root=P.Path(tempfile.mkdtemp(prefix='ux01-',dir='/tmp'))
print('Evidencia:',root,flush=True)
results=[]
for lang in ['es','en']:
 for preset in ['default','vim','override']:
  for color in ['normal','no-color']:
   label=f'{lang}-{preset}-{color}';base=root/label
   for d in ['config/maly','config/mpv','data','runtime','music','captures']: (base/d).mkdir(parents=True,exist_ok=True)
   os.chmod(base/'runtime',0o700)
   (base/'config/mpv/mpv.conf').write_text('ao=null\n')
   cfg=f'language="{lang}"\ncontrols="{preset if preset!="override" else "default"}"\nmusic_dir="{base}/music"\nupdate_check=false\nscan_durations=false\n[theme]\nbanner="off"\n[visualizer]\nenabled=false\n'
   if preset=='override':cfg+='[keys]\nfilter="f6"\nplaylists="ctrl+y"\n'
   (base/'config/maly/config.toml').write_text(cfg)
   env=os.environ.copy();env.update(XDG_CONFIG_HOME=str(base/'config'),XDG_DATA_HOME=str(base/'data'),XDG_RUNTIME_DIR=str(base/'runtime'),XDG_CACHE_HOME=str(base/'cache'),TERM='xterm-256color')
   if color=='no-color':env['NO_COLOR']='1'
   else:env.pop('NO_COLOR',None)
   sock=str(base/'tmux.sock')
   def cli(*args):return sp.run([binary,*args],env=env,text=True,capture_output=True,check=True).stdout
   def tm(*args):return sp.run(['tmux','-S',sock,*args],env=env,text=True,capture_output=True,check=True).stdout
   target='ux'
   def capture():return tm('capture-pane','-p','-t',target)
   def wait(want, absent=()):
    until=time.monotonic()+12
    while time.monotonic()<until:
     out=capture()
     if all(s in out for s in want) and all(s not in out for s in absent):return out
     time.sleep(.04)
    raise AssertionError((label,target,want,absent,out))
   def key(*k):tm('send-keys','-t',target,*k)
   def save(name):
    (base/'captures'/f'{name}.txt').write_text(capture())
    (base/'captures'/f'{name}.ansi').write_text(tm('capture-pane','-p','-e','-t',target))
   no='sin coincidencias' if lang=='es' else 'no matches';clear='esc: limpiar filtro' if lang=='es' else 'esc: clear filter';edit='Edita el filtro de arriba' if lang=='es' else 'Edit the filter above';emptypl='no hay playlists' if lang=='es' else 'no playlists';libempty='biblioteca vacía' if lang=='es' else 'library empty';qempty='cola vacía' if lang=='es' else 'queue empty'
   plkey='C-y' if preset=='override' else 'C-l';filterkey='F6' if preset=='override' else '/'
   tm('new-session','-d','-s','ux','-x','140','-y','42',binary)
   try:
    # Primer arranque real: el demonio crea la DB y la lectura vacía conserva su flash.
    wait([libempty,qempty]);save('00-first-start')
    # Fixtures sin red, con título largo, NFD, CJK, repetidos y campos ausentes.
    fixtures=[('Luz de octubre','Áurea','Horizontes'),('Canción muy larga — versión acústica para leer en una terminal pequeña','Áurea',''),('Cafe\u0301 北の空','',''),('Luz de octubre','Atlas','Niebla')]
    for i,(title,artist,album) in enumerate(fixtures):
     sp.run(['ffmpeg','-v','error','-f','lavfi','-i','anullsrc=r=8000:cl=mono','-t','120','-metadata',f'title={title}','-metadata',f'artist={artist}','-metadata',f'album={album}','-y',str(base/f'music/{i}.flac')],check=True)
    (base/'music/0.lrc').write_text('[00:00.00]Luz de octubre\n[00:02.00]Una letra de prueba\n')
    cli('scan');wait(['Áurea']);cli('add',str(base/'music/0.flac'));cli('pause')
    for w,h in [(140,42),(100,28),(80,24),(40,12),(140,42)]:
     tm('resize-window','-t',target,'-x',str(w),'-y',str(h));wait(['Áurea'])
     key(filterkey);key('-l','zzzzsincoincidencias')
     wait([no,clear]);save(f'{w}x{h}-library-edit')
     key('Enter');wait([no,clear]);save(f'{w}x{h}-library-zero')
     key('Escape');wait(['Áurea'],[no])
     key('Tab');wait(['Luz de octubre'])
     key(filterkey);key('-l','zzzzsincoincidencias');wait([no,clear]);key('Enter');wait([no,clear],[qempty]);save(f'{w}x{h}-queue-zero')
     key('Escape');wait(['Luz de octubre'],[no]);key('Tab');wait(['Áurea'])
     key(plkey);wait([emptypl,'ctrl+n'],[libempty,'scan']);save(f'{w}x{h}-no-playlists')
     key('-l','Favoritas');wait([emptypl,'ctrl+n'],[no]);key('Escape')
     key('C-o');wait(['Luz de octubre']);key('-l','zzzzsincoincidencias');wait([no,edit],[libempty]);save(f'{w}x{h}-songs-zero')
     key('C-e');key('C-w');wait(['Luz de octubre'],[no]);key('Escape')
     results.append({'run':label,'size':f'{w}x{h}','states':'library/queue filtered + clear, playlists empty + typed, songs filtered + edit','ok':True})
    key(plkey);wait([emptypl]);key('-l','Favoritas');key('C-n');wait(['Favoritas  (0)'],[emptypl]);save('playlist-created')
    key('-l','zzzzsincoincidencias');wait([no,edit],[emptypl,libempty]);save('playlist-zero');key('C-e');key('C-w');wait(['Favoritas  (0)'],[no])
    # Segundo cliente, adjunto al demonio de prueba: refresco por push.
    tm('new-session','-d','-s','peer','-x','100','-y','28',binary)
    target='peer';wait(['Áurea']);key(plkey);wait(['Favoritas  (0)'])
    cli('playlist','create','Otra');wait(['Otra  (0)']);save('peer-refresh')
    target='ux';wait(['Otra  (0)']);save('embedded-refresh')
    # Consumidores remotos: abrir/cerrar sin red, el estado propio se conserva.
    key('Escape');key('C-g');wait(['escribe qué quieres' if lang=='es' else 'type what you want'],[libempty,emptypl]);save('get-idle');key('Escape')
   finally:
    target='peer'
    try:key('C-c')
    except sp.CalledProcessError:pass
    target='ux';key('C-c')
    until=time.monotonic()+10
    while (base/'runtime/maly/maly.sock').exists() and time.monotonic()<until:time.sleep(.04)
    if (base/'runtime/maly/maly.sock').exists():raise AssertionError('No cerró el demonio de prueba')
   (root/'matrix-results.json').write_text(json.dumps(results,indent=2))
   print(label,'OK',flush=True)
(root/'matrix-results.json').write_text(json.dumps(results,indent=2))

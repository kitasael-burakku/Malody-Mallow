from control import *
import signal,hashlib
isolated()
cfg=root/'sandbox/cfg/maly/config.toml'
base=cfg.read_text().split('[keys]')[0]
for lang,preset,nocolor in [('es','default',False),('en','vim',False),('en','vim',True)]:
 content=base.replace('language = "en"',f'language = "{lang}"').replace('controls = "vim"',f'controls = "{preset}"')
 if lang=='en':content+='\n[keys]\nundo_remove="ctrl+y"\n'
 cfg.write_text(content)
 launch='launch-nocolor.py' if nocolor else 'launch.py'
 run('kitty','--detach','--class','maly-polish-review','--title','Maly · ayuda 1.22.11','--listen-on','unix:'+str(root/'kitty2.sock'),'-o','allow_remote_control=socket-only','python3',str(root/launch))
 until(lambda:(root/'kitty2.sock').exists());time.sleep(1)
 c=client();run('hyprctl','dispatch','hl.dsp.window.move({workspace="9",window="address:'+c['address']+'"})')
 size(240,52);text('?')
 if nocolor:
  capture('14-help-no-color');remote('set-background-opacity','1');remote('set-colors','background=#f5f5f5','foreground=#101010');time.sleep(1);capture('15-help-no-color-light')
 elif lang=='es':
  capture('01-help-wide-es');size(140,42);capture('02-help-140-es');size(100,28);capture('03-help-100-es');size(40,12);capture('04-help-40-es');key('escape');size(39,11);capture('05-minimum-es');size(100,28);key('ctrl+p');text('undo');key('enter');waitword('sustituida');capture('06-error-wrapped')
  text('get http://127.0.0.1:18779/NoExiste.mp3');key('enter');waitword('HTTP Error 404');capture('07-download-404-retained')
  text('get http://127.0.0.1:18779/Prueba.mp3');key('enter');waitword('Archivos de descarga');capture('08-download-success');key('escape');key('ctrl+p');key('ctrl+home');capture('09-console-scroll-start');key('escape');text('?');size(40,12);key(*(['page_down']*25));capture('10-help-bottom')
 else:
  capture('11-help-wide-en-vim');size(140,42);capture('12-help-140-en-vim');key('escape');key('ctrl+p');text('get playlist http://127.0.0.1:18779/NoExiste.mp3 QAfallo');key('enter');waitword('HTTP Error 404');capture('13-playlist-error-retained')
 key('ctrl+c');until(lambda:not (root/'kitty2.sock').exists())
os.kill(int((root/'http.pid').read_text()),signal.SIGTERM)
(dest/'visual-final.json').write_text(json.dumps({'binary_sha256':hashlib.sha256(P.Path('/tmp/maly-polish-after').read_bytes()).hexdigest(),'profiles':['es/default/color','en/vim/ctrl+y/color','en/vim/ctrl+y/NO_COLOR'],'captures':15,'result':'passed'},indent=2))
clients=json.loads(run('hyprctl','-j','clients'))
# Restaurar la conversación si sigue existiendo, sin tocar otras ventanas.
if any(c['address']=='0x5653ffc918e0' for c in clients):run('hyprctl','dispatch','hl.dsp.focus({window="address:0x5653ffc918e0"})')
print(run('pgrep','-a','-x','maly'))

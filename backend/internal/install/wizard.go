package install

import (
	"net/http"
	"os"

	"likeadmin/backend/internal/config"

	"github.com/gin-gonic/gin"
)

const wizardHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8"/>
<title>likeadmin 安装</title>
<style>
body{font-family:sans-serif;max-width:640px;margin:40px auto;color:#333}
label{display:block;margin:10px 0 4px}
input[type=text],input[type=password],input[type=number]{width:100%;padding:8px;box-sizing:border-box}
button{margin-top:16px;padding:8px 16px}
.err{color:#c00} .ok{color:#090}
.item{display:flex;justify-content:space-between;padding:4px 0;border-bottom:1px solid #eee}
fieldset{border:1px solid #eee;padding:12px;margin:16px 0}
.hint{color:#666;font-size:13px;margin:4px 0 8px;line-height:1.4}
.inline label{display:inline;margin-right:16px}
</style>
</head>
<body>
<h1>likeadmin 安装</h1>
<div id="env"></div>
<form id="f">
<fieldset>
<legend>部署模式</legend>
<div class="inline">
<label><input type="radio" name="deploy_mode" value="single" checked/> 单实例</label>
<label><input type="radio" name="deploy_mode" value="multi"/> 多实例</label>
</div>
<p class="hint">单实例可单机单库运行。多实例需要共享 Redis，上传文件请使用共享目录或安装后在后台配置 OSS；本地磁盘不能在多台机器间共享。</p>
<div id="redisBox" style="display:none">
<label>Redis 主机</label><input name="redis_host" value="127.0.0.1"/>
<label>Redis 端口</label><input name="redis_port" value="6379"/>
<label>Redis 密码</label><input name="redis_password" type="password"/>
<label>CDN 域名（可选，本地上传可留空）</label><input name="cdn_domain" placeholder="https://cdn.example.com"/>
</div>
</fieldset>
<fieldset>
<legend>数据库</legend>
<div class="inline">
<label><input type="radio" name="db_mode" value="single" checked/> 单数据库</label>
<label><input type="radio" name="db_mode" value="replica"/> 主从库</label>
</div>
<p class="hint">单数据库把读写都打到下面的主库。主从库只把可延迟的读（日志、导出、统计）打到从库；安装、建表、管理员种子只写主库。</p>
<label>数据库主机</label><input name="host" value="127.0.0.1"/>
<label>端口</label><input name="port" value="3306"/>
<label>用户名</label><input name="user" value="root"/>
<label>密码</label><input name="password" type="password"/>
<label>数据库名</label><input name="name" value="likeadmin_saas"/>
<label>表前缀</label><input name="prefix" value="la_"/>
<div id="replicaBox" style="display:none">
<label>从库主机</label><input name="replica_host"/>
<label>从库端口</label><input name="replica_port" value="3306"/>
<label>从库用户名（可留空，默认与主库相同）</label><input name="replica_user"/>
<label>从库密码（可留空，默认与主库相同）</label><input name="replica_password" type="password"/>
</div>
</fieldset>
<label>管理员账号</label><input name="admin_user"/>
<label>管理员密码</label><input name="admin_password" type="password"/>
<label>确认密码</label><input name="admin_confirm_password" type="password"/>
<label><input type="checkbox" name="clear_db"/> 清空已有数据库</label>
<label><input type="checkbox" name="import_test_data"/> 导入测试数据</label>
<button type="submit">开始安装</button>
</form>
<pre id="msg"></pre>
<script>
function syncBoxes(){
  const multi=document.querySelector('[name=deploy_mode]:checked').value==='multi';
  document.getElementById('redisBox').style.display=multi?'block':'none';
  const replica=document.querySelector('[name=db_mode]:checked').value==='replica';
  document.getElementById('replicaBox').style.display=replica?'block':'none';
}
document.querySelectorAll('[name=deploy_mode],[name=db_mode]').forEach(el=>el.addEventListener('change',syncBoxes));
syncBoxes();
fetch('/install/env').then(r=>r.json()).then(j=>{
  const d=j.data||{};
  const box=document.getElementById('env');
  (d.items||[]).forEach(it=>{
    const div=document.createElement('div');
    div.className='item';
    div.innerHTML='<span>'+it.name+'</span><span class="'+(it.status==='ok'?'ok':'err')+'">'+it.status+' '+ (it.value||'')+'</span>';
    box.appendChild(div);
  });
  if(d.installed){document.getElementById('msg').textContent='程序已安装';}
});
document.getElementById('f').onsubmit=async ev=>{
  ev.preventDefault();
  const fd=new FormData(ev.target);
  const body={};
  fd.forEach((v,k)=>body[k]=v);
  body.clear_db=document.querySelector('[name=clear_db]').checked?'on':'off';
  body.import_test_data=document.querySelector('[name=import_test_data]').checked?'on':'off';
  const res=await fetch('/install',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  const j=await res.json();
  document.getElementById('msg').textContent=(j.msg||'')+' '+JSON.stringify(j.data||{});
};
</script>
</body>
</html>`

func Wizard(c *gin.Context) {
	// PHP install.php die()s this exact string when install.lock exists.
	if lock := config.C.App.InstallLock; lock != "" {
		if _, err := os.Stat(lock); err == nil {
			c.String(http.StatusOK, installedMsg)
			return
		}
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(wizardHTML))
}

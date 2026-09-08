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
input{width:100%;padding:8px;box-sizing:border-box}
button{margin-top:16px;padding:8px 16px}
.err{color:#c00} .ok{color:#090}
.item{display:flex;justify-content:space-between;padding:4px 0;border-bottom:1px solid #eee}
</style>
</head>
<body>
<h1>likeadmin 安装</h1>
<div id="env"></div>
<form id="f">
<label>数据库主机</label><input name="host" value="127.0.0.1"/>
<label>端口</label><input name="port" value="3306"/>
<label>用户名</label><input name="user" value="root"/>
<label>密码</label><input name="password" type="password"/>
<label>数据库名</label><input name="name" value="likeadmin_saas"/>
<label>表前缀</label><input name="prefix" value="la_"/>
<label>管理员账号</label><input name="admin_user"/>
<label>管理员密码</label><input name="admin_password" type="password"/>
<label>确认密码</label><input name="admin_confirm_password" type="password"/>
<label><input type="checkbox" name="clear_db"/> 清空已有数据库</label>
<label><input type="checkbox" name="import_test_data"/> 导入测试数据</label>
<button type="submit">开始安装</button>
</form>
<pre id="msg"></pre>
<script>
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

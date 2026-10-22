package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

var PORT = 8690 // 每次启动随机（10240~49150），防固定端口扫描

var sendDir string // 快传文件夹：电脑放文件给手机下，手机上传也直接进来

const pageHTML = `<!doctype html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>快传助手</title>
<style>
*{box-sizing:border-box;margin:0}
body{font-family:-apple-system,"PingFang SC",Arial,sans-serif;background:#f2f5fb;color:#1c2333;padding:18px}
h1{font-size:20px;text-align:center;margin:6px 0 14px}
.tool{display:flex;gap:10px;align-items:center;margin-bottom:12px}
.tool label{font-size:14px;display:flex;align-items:center;gap:6px}
button.dl{background:#2b5bd7;color:#fff;border:none;border-radius:6px;padding:6px 14px;font-size:14px}
.card{background:#fff;border-radius:10px;overflow:hidden;box-shadow:0 1px 4px rgba(0,0,0,.08)}
ul{list-style:none;padding:0}
li{display:flex;align-items:center;gap:10px;padding:12px 14px;border-bottom:1px solid #eef1f7}
li:last-child{border-bottom:none}
.cb{width:18px;height:18px;flex:none}
.nm{flex:1;font-size:14px;word-break:break-all}
.sz{color:#8a93a5;font-size:12px;flex:none}
a.dl{color:#2b5bd7;text-decoration:none;font-size:14px;flex:none}
label.auto{margin-left:auto}
#prog{display:none;background:#fff3e0;color:#8a5b00;border:1px solid #ffd591;border-radius:8px;padding:8px 12px;font-size:13px;margin-bottom:12px}
.upbar{display:flex;align-items:center;gap:10px;margin:12px 0}
button.up{background:#2ea44f;color:#fff;border:none;border-radius:6px;padding:7px 16px;font-size:14px}
#uprog{display:none;background:#e6f6ec;color:#116b36;border:1px solid #b7e3c6;border-radius:8px;padding:8px 12px;font-size:13px;margin-bottom:12px}
.empty{color:#8a93a5;font-size:14px;line-height:1.8;text-align:center;padding:26px 14px;display:block}
</style>
</head><body>
<h1>快传助手</h1>
<div class="tool">
  <label><input type="checkbox" id="all" onclick="toggleAll(this)"> 全选</label>
  <button class="dl" onclick="batchDownload()">批量下载</button>
  <label class="auto"><input type="checkbox" id="auto" checked> 自动接收</label>
</div>
<div id="prog"></div>
<div class="card"><ul id="list">__ROWS__</ul></div>
<div class="upbar">
  <button class="up" onclick="document.getElementById('upfile').click()">上传到电脑</button>
  <input type="file" id="upfile" multiple style="display:none" onchange="upPick(this)">
</div>
<div id="uprog"></div>
<script>
function toggleAll(c){ var cbs=document.querySelectorAll('.cb'); for(var i=0;i<cbs.length;i++){ cbs[i].checked=c.checked; } }
function hsize(b){ if(b<1024)return b+' B'; var u=['KB','MB','GB']; var i=-1; do{b/=1024;i++;}while(b>=1024&&i<2); return b.toFixed(1)+' '+u[i]; }

// ---- 下载队列：按顺序、每次 2 个小批（黑屏也保持顺序不乱），带进度防漏 ----
var dlQ=[], dlTimer=null, dlTotal=0;
function showProg(){
  var el=document.getElementById('prog');
  el.style.display='block';
  el.textContent='正在下载：共 '+dlTotal+' 个，还剩 '+dlQ.length+' 个';
}
function progDone(){
  var el=document.getElementById('prog');
  el.textContent='共 '+dlTotal+' 个已全部开始下载';
  setTimeout(function(){ el.style.display='none'; }, 4000);
}
function markSent(nm){
  var cbs=document.querySelectorAll('.cb');
  for(var i=0;i<cbs.length;i++){ if(cbs[i].dataset.name===nm){ cbs[i].checked=false; } }
}
function enqueue(names){
  var wasEmpty=dlQ.length===0;
  for(var i=0;i<names.length;i++){ if(dlQ.indexOf(names[i])<0){ dlQ.push(names[i]); } }
  if(wasEmpty){ dlTotal=dlQ.length; showProg(); }
  if(!dlTimer){ pump(); }
}
function pump(){
  if(!dlQ.length){ dlTimer=null; progDone(); return; }
  var take=dlQ.splice(0,2);
  for(var k=0;k<take.length;k++){ nativeDl(take[k]); markSent(take[k]); }
  showProg();
  dlTimer=setTimeout(pump, 2500);
}
function nativeDl(nm){
  var a=document.createElement('a');
  a.href='/f/'+nm; a.download=decodeURIComponent(nm);
  document.body.appendChild(a); a.click();
  setTimeout(function(el){ el.remove(); }, 3000, a);
}
function batchDownload(){
  var cbs=document.querySelectorAll('.cb:checked');
  if(!cbs.length){ alert('请先勾选要下载的文件'); return; }
  enqueue(Array.prototype.map.call(cbs,function(c){ return c.dataset.name; }));
}

// ---- 自动接收 + 实时刷新 ----
var lastKey='', lastNames={}, firstRun=true;
function refreshList(){
  fetch('/refresh').then(function(r){ return r.json(); }).then(function(arr){
    var fresh=[];
    for(var i=0;i<arr.length;i++){ if(!lastNames[arr[i].n]){ fresh.push(arr[i].n); } }
    lastNames={}; for(var j=0;j<arr.length;j++){ lastNames[arr[j].n]=true; }
    if(firstRun){ firstRun=false; }
    else if(document.getElementById('auto').checked && fresh.length){ enqueue(fresh); }
    var key=arr.map(function(x){ return x.n; }).join('|');
    if(key===lastKey) return; lastKey=key;
    var checked={}; var cbs=document.querySelectorAll('.cb:checked');
    for(var k=0;k<cbs.length;k++){ checked[cbs[k].dataset.name]=true; }
    var ul=document.querySelector('#list');
    if(!arr.length){ ul.innerHTML='<li class="empty">快传文件夹还没有文件。</li>'; return; }
    var h='';
    for(var m=0;m<arr.length;m++){
      var x=arr[m];
      h+='<li><input type="checkbox" class="cb" data-size="'+x.s+'" data-name="'+x.n+'"'+((checked[x.n])?' checked':'')+'>'
        +'<div class="nm" title="'+x.d+'"></div><div class="sz"></div>'
        +'<a class="dl" href="/f/'+x.n+'" download>下载</a></li>';
    }
    ul.innerHTML=h;
    var lis=ul.querySelectorAll('li');
    for(var t=0;t<arr.length;t++){
      lis[t].querySelector('.nm').textContent=arr[t].d;
      lis[t].querySelector('.sz').textContent=hsize(arr[t].s);
    }
  }).catch(function(){});
}
// ---- 手机上传到电脑（分块，支持大视频）----
var UP_CHUNK=4*1024*1024, upQ=[], upBusy=false;
function upPick(inp){
  var files=Array.prototype.slice.call(inp.files||[]);
  if(!files.length) return;
  inp.value='';
  upQ=upQ.concat(files);
  if(!upBusy){ upNext(); }
}
function upNext(){
  if(!upQ.length){ upBusy=false; var el=document.getElementById('uprog'); el.style.display='none'; return; }
  upBusy=true;
  upFile(upQ.shift());
}
function upFile(file){
  var name=file.name, total=file.size, n=Math.max(1,Math.ceil(total/UP_CHUNK));
  var i=0;
  function show(t){ var el=document.getElementById('uprog'); el.style.display='block'; el.textContent='正在上传：'+name+' ('+t+'/'+n+')'; }
  show(0);
  function next(){
    if(i>=n){ show(n); setTimeout(upNext,300); return; }
    var start=i*UP_CHUNK, end=Math.min(start+UP_CHUNK,total);
    fetch('/up?n='+encodeURIComponent(name)+'&off='+start,{method:'POST',body:file.slice(start,end)})
      .then(function(r){ if(!r.ok){ throw 0; } i++; show(i); next(); })
      .catch(function(){ alert('上传中断：'+name); upNext(); });
  }
  next();
}
setInterval(refreshList, 2000);
</script>
</body></html>`

type FileItem struct {
	N string `json:"n"` // url-encoded name (for href / data-name)
	S int64  `json:"s"` // size bytes
	D string `json:"d"` // display name
}

func listFiles() []FileItem {
	items := []FileItem{}
	entries, err := os.ReadDir(sendDir)
	if err != nil {
		return items
	}
	for _, e := range entries {
		if e.IsDir() { // 只列文件，跳过子目录
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := e.Name()
		items = append(items, FileItem{N: url.PathEscape(name), S: info.Size(), D: name})
	}
	return items
}

func humanSize(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10) + " B"
	}
	units := []string{"KB", "MB", "GB"}
	v := float64(n)
	u := 0
	for v >= 1024 && u < len(units)-1 {
		v /= 1024
		u++
	}
	return fmt.Sprintf("%.1f %s", v, units[u])
}

func buildRows() string {
	var b strings.Builder
	for _, it := range listFiles() {
		esc := html.EscapeString(it.D)
		b.WriteString(`<li><input type="checkbox" class="cb" data-size="`)
		b.WriteString(strconv.FormatInt(it.S, 10))
		b.WriteString(`" data-name="`)
		b.WriteString(html.EscapeString(it.N))
		b.WriteString(`"><div class="nm" title="`)
		b.WriteString(esc)
		b.WriteString(`">`)
		b.WriteString(esc)
		b.WriteString(`</div><div class="sz">`)
		b.WriteString(humanSize(it.S))
		b.WriteString(`</div><a class="dl" href="/f/`)
		b.WriteString(it.N)
		b.WriteString(`" download>下载</a></li>`)
	}
	if b.Len() == 0 {
		return `<li class="empty">快传文件夹还没有文件。</li>`
	}
	return b.String()
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" && r.URL.Path == "/up" {
		handleUpload(w, r)
		return
	}
	p := r.URL.Path
	switch {
	case p == "/" || p == "":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		io.WriteString(w, strings.Replace(pageHTML, "__ROWS__", buildRows(), 1))
	case p == "/refresh":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(listFiles())
	case strings.HasPrefix(p, "/f/"):
		serveFile(w, r, p[len("/f/"):])
	default:
		http.NotFound(w, r)
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	esc := q.Get("n")
	offStr := q.Get("off")
	name, err := url.PathUnescape(esc)
	if err != nil {
		http.Error(w, "bad name", 400)
		return
	}
	name = filepath.Base(name) // 防目录穿越
	if name == "" || name == "." || name == ".." {
		http.Error(w, "bad name", 400)
		return
	}
	off := int64(0)
	if offStr != "" {
		if v, e := strconv.ParseInt(offStr, 10, 64); e == nil && v > 0 {
			off = v
		}
	}
	data, err := io.ReadAll(r.Body)
	if err != nil || len(data) == 0 {
		http.Error(w, "no data", 400)
		return
	}
	full := filepath.Join(sendDir, name)
	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "open fail", 500)
		return
	}
	defer f.Close()
	if _, err := f.WriteAt(data, off); err != nil {
		http.Error(w, "write fail", 500)
		return
	}
	w.WriteHeader(200)
	io.WriteString(w, "ok")
}

func serveFile(w http.ResponseWriter, r *http.Request, esc string) {
	name, err := url.PathUnescape(esc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(sendDir, name)
	// 防止目录穿越
	absSend, _ := filepath.Abs(sendDir)
	absFull, _ := filepath.Abs(full)
	if !(strings.HasPrefix(absFull, absSend+string(os.PathSeparator))) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(absFull)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	// 中文文件名下载：RFC5987 filename*
	fallback := strings.ReplaceAll(name, `"`, "")
	dispo := "attachment; filename=\"" + fallback + "\"; filename*=UTF-8''" + url.PathEscape(name)
	w.Header().Set("Content-Disposition", dispo)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	http.ServeContent(w, r, name, info.ModTime(), mustOpen(absFull))
}

func mustOpen(p string) *os.File {
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	return f
}

func lanIP() string {
	ifs, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	best := ""
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipn.IP.To4()
			if ip == nil {
				continue
			}
			if ip[0] == 192 && ip[1] == 168 {
				return ip.String()
			}
			if ip[0] == 10 || (ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31) {
				if best == "" {
					best = ip.String()
				}
			}
		}
	}
	if best != "" {
		return best
	}
	return "127.0.0.1"
}

func main() {
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		exeDir, _ = os.Getwd()
	}
	sendDir = filepath.Join(exeDir, "快传")
	os.MkdirAll(sendDir, 0755)

	// 每次启动随机端口，避免固定端口被扫描
	var pb [2]byte
	rand.Read(pb[:])
	v := int(binary.BigEndian.Uint16(pb[:]))
	PORT = 10240 + v%(49151-10240)

	ip := lanIP()
	urlStr := fmt.Sprintf("http://%s:%d", ip, PORT)

	fmt.Println("========================================================")
	fmt.Println("  快传助手 —— WiFi 直传已开启")
	fmt.Println("========================================================")
	fmt.Println("快传文件夹：" + sendDir)
	fmt.Println("电脑把文件放进去，手机就能下载；")
	fmt.Println("手机上传的文件也直接存进这个文件夹。")
	fmt.Println()
	fmt.Println("手机操作：连同一个 WiFi，用浏览器打开下面网址：")
	fmt.Println("  " + urlStr)
	fmt.Println("本次端口：" + strconv.Itoa(PORT) + "（每次启动都会变）")
	fmt.Println("如果 Windows 弹出防火墙提示，请点“允许访问”。")
	if q, err := qrcode.New(urlStr, qrcode.Low); err == nil {
		bm := q.Bitmap()
		rows := len(bm)
		cols := len(bm[0])
		var sb strings.Builder
		for r := 0; r < rows; r += 2 {
			for c := 0; c < cols; c++ {
				top := bm[r][c]
				bot := false
				if r+1 < rows {
					bot = bm[r+1][c]
				}
				switch {
				case top && bot:
					sb.WriteString("█")
				case top && !bot:
					sb.WriteString("▀")
				case !top && bot:
					sb.WriteString("▄")
				default:
					sb.WriteString(" ")
				}
			}
			sb.WriteString("\n")
		}
		fmt.Print(sb.String())
	} else {
		fmt.Println("  （二维码生成失败，请手动输入上面的网址）")
	}
	fmt.Println()
	fmt.Println("关闭本窗口即可停止。")

	// 忽略 Ctrl+C（误按不会退出），停止靠关闭窗口
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for range sigCh {
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	srv := &http.Server{
		Addr:           "0.0.0.0:" + strconv.Itoa(PORT),
		Handler:        mux,
		ReadTimeout:    0,
		WriteTimeout:   0,
		IdleTimeout:    30 * time.Second,
	}
	srv.ListenAndServe()
}

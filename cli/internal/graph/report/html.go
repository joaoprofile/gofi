package report

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/graph/analyze"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

type vizNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Pkg   string `json:"pkg"`
	LOC   int    `json:"loc"`
	Syms  int    `json:"syms"`
	In    int    `json:"in"`
	Out   int    `json:"out"`
	Comm  int    `json:"comm"`
}

type vizLink struct {
	S int `json:"s"`
	T int `json:"t"`
}

type vizHub struct {
	Label string `json:"label"`
	Kind  string `json:"kind"`
	Loc   string `json:"loc"`
	In    int    `json:"in"`
	Out   int    `json:"out"`
	Pkg   string `json:"pkg"`
}

type vizData struct {
	Module  string     `json:"module"`
	Stack   string     `json:"stack"`
	Mode    string     `json:"mode"`
	Nodes   []vizNode  `json:"nodes"`
	Links   []vizLink  `json:"links"`
	Hubs    []vizHub   `json:"hubs"`
	Stats   [][]string `json:"stats"`
	Omitted int        `json:"omitidos"`
}

// HTML renders a single self-contained page with the package map. No CDN, no
// network, no local storage: it opens offline and works on any machine.
func HTML(g *model.Graph) (string, error) {
	// Each package inherits the dominant community among the symbols it holds.
	pkgCommunities := map[string]map[int]int{}
	symbolCount := map[string]int{}
	for _, e := range g.Edges {
		if e.Rel != model.RelContains {
			continue
		}
		from, to := g.Get(e.From), g.Get(e.To)
		if from == nil || to == nil || from.Kind != model.KindPackage {
			continue
		}
		symbolCount[from.ID]++
		if pkgCommunities[from.ID] == nil {
			pkgCommunities[from.ID] = map[int]int{}
		}
		pkgCommunities[from.ID][to.Community]++
	}

	var pkgs []*model.Node
	for _, n := range g.Nodes {
		if n.Kind == model.KindPackage && !n.External {
			pkgs = append(pkgs, n)
		}
	}
	// Past a few hundred nodes the drawing turns to soup and the simulation gets
	// slow. We cut the least relevant ones and report how many were left out,
	// instead of truncating silently.
	const maxVizNodes = 500
	omitted := 0
	if len(pkgs) > maxVizNodes {
		slices.SortFunc(pkgs, func(a, b *model.Node) int {
			pa := a.InDeg*3 + a.OutDeg + a.Lines/200
			pb := b.InDeg*3 + b.OutDeg + b.Lines/200
			return cmp.Or(
				cmp.Compare(pb, pa),
				cmp.Compare(a.ID, b.ID),
			)
		})
		omitted = len(pkgs) - maxVizNodes
		pkgs = pkgs[:maxVizNodes]
	}
	slices.SortFunc(pkgs, func(a, b *model.Node) int { return cmp.Compare(a.ID, b.ID) })

	idx := map[string]int{}
	d := vizData{Module: g.Module, Stack: stack(g), Mode: g.Mode}
	for i, p := range pkgs {
		idx[p.ID] = i
		comm, best := 0, -1
		for c, n := range pkgCommunities[p.ID] {
			if n > best || (n == best && c < comm) {
				comm, best = c, n
			}
		}
		d.Nodes = append(d.Nodes, vizNode{
			ID: p.ID, Label: model.ShortID(p.ID, g.Module), Pkg: p.Unit,
			LOC: p.Lines, Syms: symbolCount[p.ID], In: p.InDeg, Out: p.OutDeg, Comm: comm,
		})
	}
	for _, e := range g.Edges {
		if e.Rel != model.RelImports {
			continue
		}
		s, ok1 := idx[e.From]
		t, ok2 := idx[e.To]
		if ok1 && ok2 && s != t {
			d.Links = append(d.Links, vizLink{S: s, T: t})
		}
	}
	for _, n := range analyze.Hubs(g, 24, false) {
		loc := n.File
		if loc != "" {
			loc = loc + ":" + itoa(n.Line)
		}
		d.Hubs = append(d.Hubs, vizHub{
			Label: model.ShortID(n.ID, g.Module), Kind: string(n.Kind),
			Loc: loc, In: n.InDeg, Out: n.OutDeg, Pkg: n.Unit,
		})
	}
	d.Omitted = omitted
	s := g.Stats
	d.Stats = [][]string{
		{"pacotes", itoa(s.Packages)},
		{"tipos", itoa(s.Types)},
		{"funcoes", itoa(s.Funcs)},
		{"metodos", itoa(s.Methods)},
		{"linhas", itoa(s.LOC)},
		{"arestas", itoa(s.Edges)},
		{"comunidades", itoa(s.Communities)},
	}

	b, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return strings.Replace(pageHTML, "/*__DADOS__*/", string(b), 1), nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [24]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

const pageHTML = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>gofi graph — mapa de pacotes</title>
<style>
  :root{
    --bg:#0f1216; --panel:#161b22; --line:#232b36; --fg:#e6edf3; --dim:#8b98a5;
    --accent:#4c9be8;
  }
  @media (prefers-color-scheme: light){
    :root{ --bg:#fbfcfd; --panel:#ffffff; --line:#e3e8ee; --fg:#1c2530; --dim:#5d6b7a; }
  }
  *{box-sizing:border-box}
  body{margin:0;background:var(--bg);color:var(--fg);
    font:14px/1.5 ui-sans-serif,-apple-system,"Segoe UI",Roboto,sans-serif}
  header{display:flex;align-items:baseline;gap:16px;flex-wrap:wrap;
    padding:14px 20px;border-bottom:1px solid var(--line);background:var(--panel)}
  h1{font-size:15px;margin:0;font-weight:650;letter-spacing:.2px}
  .sub{color:var(--dim);font-size:12.5px}
  .stats{display:flex;gap:18px;margin-left:auto;flex-wrap:wrap}
  .stat b{font-variant-numeric:tabular-nums;font-weight:650}
  .stat span{color:var(--dim);font-size:11.5px;text-transform:uppercase;
    letter-spacing:.6px;margin-left:5px}
  main{display:grid;grid-template-columns:1fr 320px;height:calc(100vh - 58px)}
  @media (max-width:900px){main{grid-template-columns:1fr;height:auto}}
  #wrap{position:relative;overflow:hidden}
  canvas{display:block;width:100%;height:100%;cursor:grab}
  canvas:active{cursor:grabbing}
  aside{border-left:1px solid var(--line);background:var(--panel);
    overflow-y:auto;padding:16px}
  @media (max-width:900px){aside{border-left:0;border-top:1px solid var(--line)}
    #wrap{height:60vh}}
  aside h2{font-size:11.5px;text-transform:uppercase;letter-spacing:.7px;
    color:var(--dim);margin:0 0 10px;font-weight:600}
  .hub{padding:7px 9px;border-radius:7px;margin-bottom:2px;cursor:pointer}
  .hub:hover{background:var(--line)}
  .hub .n{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12.5px;
    word-break:break-all}
  .hub .m{color:var(--dim);font-size:11.5px;margin-top:2px;
    font-variant-numeric:tabular-nums}
  input[type=search]{width:100%;padding:8px 10px;border-radius:7px;
    border:1px solid var(--line);background:var(--bg);color:var(--fg);
    font-size:13px;margin-bottom:14px}
  input[type=search]:focus{outline:2px solid var(--accent);outline-offset:-1px}
  #tip{position:absolute;pointer-events:none;background:var(--panel);
    border:1px solid var(--line);border-radius:8px;padding:8px 10px;
    font-size:12.5px;opacity:0;transition:opacity .12s;max-width:280px;
    box-shadow:0 6px 24px rgba(0,0,0,.28)}
  #tip .t{font-family:ui-monospace,Menlo,monospace;font-weight:600;
    margin-bottom:4px;word-break:break-all}
  #tip .r{color:var(--dim);font-variant-numeric:tabular-nums}
  .legend{margin-top:20px;display:flex;flex-wrap:wrap;gap:8px 14px}
  .legend div{display:flex;align-items:center;gap:6px;font-size:12px;color:var(--dim)}
  .sw{width:10px;height:10px;border-radius:3px;flex:0 0 auto}
  footer{padding:10px 20px;color:var(--dim);font-size:11.5px;
    border-top:1px solid var(--line)}
</style>
</head>
<body>
<header>
  <h1>gofi graph</h1>
  <span class="sub" id="mod"></span>
  <div class="stats" id="stats"></div>
</header>
<main>
  <div id="wrap"><canvas id="c"></canvas><div id="tip"></div></div>
  <aside>
    <input type="search" id="q" placeholder="filtrar pacotes e simbolos..." aria-label="filtrar">
    <h2>Pontos centrais</h2>
    <div id="hubs"></div>
    <div class="legend" id="legend"></div>
  </aside>
</main>
<footer id="foot"></footer>
<script>
const D = /*__DADOS__*/;
const PAL = ["#4c78a8","#f58518","#54a24b","#e45756","#72b7b2",
             "#eeca3b","#b279a2","#ff9da6","#9d755d","#79706e"];
const cor = c => PAL[((c % PAL.length) + PAL.length) % PAL.length];

document.getElementById("mod").textContent = D.module + " · " + D.stack + " · modo " + D.mode;
document.getElementById("stats").innerHTML = D.stats
  .map(([k,v]) => '<div class="stat"><b>'+v+'</b><span>'+k+'</span></div>').join("");
document.getElementById("foot").textContent =
  "Arraste os nos. O tamanho reflete o numero de linhas; a cor, a comunidade. " +
  "As ligacoes sao imports reais extraidos do codigo." +
  (D.omitidos ? "  " + D.omitidos + " pacotes menos conectados foram omitidos do desenho (continuam no grafo e nas consultas)." : "");

// ---- Fruchterman-Reingold layout, deterministic ----
// No random numbers: the starting position is a golden-angle spiral, so the
// same graph always draws the same way.
const N = D.nodes.length;
const LADO = 1000;
const K = Math.sqrt((LADO*LADO) / Math.max(N,1));   // ideal distance between nodes
// Size is normalized by the largest package: without that, a giant module like
// runtime would become a disc that swallows the whole drawing.
const MAXLOC = Math.max(1, ...D.nodes.map(n => n.loc));
const P = D.nodes.map((n,i) => {
  const a = i * 2.399963388;
  const r = K * 0.55 * Math.sqrt(i + 1);
  return {x: Math.cos(a)*r, y: Math.sin(a)*r,
          r: 3.5 + 15 * Math.sqrt(Math.max(n.loc,1) / MAXLOC), n};
});
// Only the most connected packages get a permanent label; the rest show on hover.
const rotulados = new Set(
  D.nodes.map((n,i) => [i, n.in*3 + n.out])
         .sort((a,b) => b[1]-a[1] || a[0]-b[0])
         .slice(0, 22).map(p => p[0]));
const grau = new Array(N).fill(0);
D.links.forEach(l => { grau[l.s]++; grau[l.t]++; });

(function layout(){
  const dx = new Float64Array(N), dy = new Float64Array(N);
  const ITERS = N > 300 ? 260 : 400;
  let temp = LADO * 0.12;
  const resfria = temp / (ITERS + 1);

  for (let it = 0; it < ITERS; it++){
    dx.fill(0); dy.fill(0);
    // repulsion between every pair
    for (let i = 0; i < N; i++){
      for (let j = i+1; j < N; j++){
        let ex = P[i].x - P[j].x, ey = P[i].y - P[j].y;
        let d = Math.sqrt(ex*ex + ey*ey);
        if (d < 0.01){ ex = (i - j) * 0.01 + 0.01; ey = 0.01; d = 0.02; }
        const f = (K*K) / d;
        const ux = ex/d*f, uy = ey/d*f;
        dx[i] += ux; dy[i] += uy;
        dx[j] -= ux; dy[j] -= uy;
      }
    }
    // attraction along the edges
    for (const l of D.links){
      let ex = P[l.s].x - P[l.t].x, ey = P[l.s].y - P[l.t].y;
      const d = Math.sqrt(ex*ex + ey*ey) || 0.01;
      const f = (d*d) / K;
      const ux = ex/d*f, uy = ey/d*f;
      dx[l.s] -= ux; dy[l.s] -= uy;
      dx[l.t] += ux; dy[l.t] += uy;
    }
    // Linear gravity: zero at the centre, growing with distance. Weak enough
    // not to crush the core, strong enough not to lose anyone.
    for (let i = 0; i < N; i++){
      dx[i] -= P[i].x * 0.08;
      dy[i] -= P[i].y * 0.08;
    }
    // displacement is capped by the temperature, which cools every step
    const meia = LADO / 2;
    for (let i = 0; i < N; i++){
      const d = Math.sqrt(dx[i]*dx[i] + dy[i]*dy[i]) || 0.01;
      const lim = Math.min(d, temp);
      P[i].x += dx[i]/d * lim;
      P[i].y += dy[i]/d * lim;
      // Frame: a package with no internal link rests against the border
      // instead of being flung to infinity and flattening the rest of the drawing.
      P[i].x = Math.max(-meia, Math.min(meia, P[i].x));
      P[i].y = Math.max(-meia, Math.min(meia, P[i].y));
    }
    temp -= resfria;
  }
})();

// ---- render ----
const cv = document.getElementById("c"), ctx = cv.getContext("2d");
const wrap = document.getElementById("wrap"), tip = document.getElementById("tip");
let view = {x:0, y:0, k:1}, hover = -1, sel = -1, filtro = "";

function fit(){
  const r = wrap.getBoundingClientRect();
  const dpr = window.devicePixelRatio || 1;
  cv.width = r.width*dpr; cv.height = r.height*dpr;
  ctx.setTransform(dpr,0,0,dpr,0,0);
  let minx=1e9,maxx=-1e9,miny=1e9,maxy=-1e9;
  for (const p of P){
    minx=Math.min(minx,p.x-p.r); maxx=Math.max(maxx,p.x+p.r);
    miny=Math.min(miny,p.y-p.r); maxy=Math.max(maxy,p.y+p.r);
  }
  const w = maxx-minx || 1, h = maxy-miny || 1;
  view.k = Math.min(r.width/(w+90), r.height/(h+90), 2.4);
  view.x = r.width/2 - (minx+maxx)/2*view.k;
  view.y = r.height/2 - (miny+maxy)/2*view.k;
  draw();
}
const sx = p => p.x*view.k + view.x;
const sy = p => p.y*view.k + view.y;

function visivel(i){
  if (!filtro) return true;
  return P[i].n.label.toLowerCase().includes(filtro);
}
function vizinho(i){
  if (sel < 0) return false;
  if (i === sel) return true;
  return D.links.some(l => (l.s===sel && l.t===i) || (l.t===sel && l.s===i));
}

function draw(){
  const r = wrap.getBoundingClientRect();
  ctx.clearRect(0,0,r.width,r.height);
  const escuro = matchMedia("(prefers-color-scheme: dark)").matches;

  for (const l of D.links){
    const a = P[l.s], b = P[l.t];
    const ativo = sel<0 ? true : (l.s===sel || l.t===sel);
    const vis = visivel(l.s) && visivel(l.t);
    ctx.globalAlpha = !vis ? 0.04 : (ativo ? (sel<0?0.20:0.75) : 0.05);
    ctx.strokeStyle = ativo && sel>=0 ? cor(P[sel].n.comm) : (escuro ? "#8b98a5" : "#8494a6");
    ctx.lineWidth = ativo && sel>=0 ? 1.6 : 1;
    ctx.beginPath();
    ctx.moveTo(sx(a),sy(a));
    // a slight curve keeps the two directions apart
    const mx = (sx(a)+sx(b))/2 + (sy(b)-sy(a))*0.06;
    const my = (sy(a)+sy(b))/2 - (sx(b)-sx(a))*0.06;
    ctx.quadraticCurveTo(mx,my,sx(b),sy(b));
    ctx.stroke();
  }
  ctx.globalAlpha = 1;

  for (let i=0;i<N;i++){
    const p = P[i];
    const vis = visivel(i), rel = sel<0 || vizinho(i);
    ctx.globalAlpha = !vis ? 0.12 : (rel ? 1 : 0.22);
    ctx.beginPath();
    ctx.arc(sx(p), sy(p), Math.max(p.r*view.k, 3.5), 0, 6.284);
    ctx.fillStyle = cor(p.n.comm);
    ctx.fill();
    if (i===hover || i===sel){
      ctx.lineWidth = 2;
      ctx.strokeStyle = escuro ? "#e6edf3" : "#1c2530";
      ctx.stroke();
    }
    const rotula = rotulados.has(i) || i===hover || i===sel;
    if (rotula && vis){
      ctx.globalAlpha = rel ? 1 : 0.35;
      ctx.font = "11px ui-monospace,Menlo,monospace";
      ctx.textAlign = "center";
      const tx = sx(p), ty = sy(p) - Math.max(p.r*view.k,4) - 6;
      // outline in the background colour: the label stays readable over edges
      ctx.lineWidth = 3;
      ctx.strokeStyle = escuro ? "#0f1216" : "#fbfcfd";
      ctx.strokeText(p.n.label, tx, ty);
      ctx.fillStyle = escuro ? "#e6edf3" : "#1c2530";
      ctx.fillText(p.n.label, tx, ty);
    }
  }
  ctx.globalAlpha = 1;
}

function acha(mx,my){
  for (let i=N-1;i>=0;i--){
    const p = P[i];
    const d = Math.hypot(mx-sx(p), my-sy(p));
    if (d <= Math.max(p.r*view.k,5)+3) return i;
  }
  return -1;
}

let arrasto = null;
cv.addEventListener("mousemove", ev => {
  const r = cv.getBoundingClientRect();
  const mx = ev.clientX-r.left, my = ev.clientY-r.top;
  if (arrasto !== null){
    P[arrasto].x = (mx - view.x)/view.k;
    P[arrasto].y = (my - view.y)/view.k;
    draw(); return;
  }
  const i = acha(mx,my);
  if (i !== hover){ hover = i; draw(); }
  if (i >= 0){
    const n = P[i].n;
    tip.innerHTML = '<div class="t">'+n.label+'</div><div class="r">'+
      n.syms+' simbolos · '+n.loc+' linhas<br>'+
      n.in+' dependem dele · depende de '+n.out+'<br>comunidade C'+n.comm+'</div>';
    tip.style.opacity = 1;
    tip.style.left = Math.min(mx+14, r.width-290)+"px";
    tip.style.top = (my+14)+"px";
  } else tip.style.opacity = 0;
});
cv.addEventListener("mousedown", ev => {
  const r = cv.getBoundingClientRect();
  const i = acha(ev.clientX-r.left, ev.clientY-r.top);
  if (i>=0){ arrasto = i; sel = i; draw(); }
  else { sel = -1; draw(); }
});
addEventListener("mouseup", () => { arrasto = null; });
cv.addEventListener("wheel", ev => {
  ev.preventDefault();
  const r = cv.getBoundingClientRect();
  const mx = ev.clientX-r.left, my = ev.clientY-r.top;
  const k = Math.min(Math.max(view.k * (ev.deltaY<0?1.12:0.89), 0.15), 6);
  view.x = mx - (mx-view.x)*(k/view.k);
  view.y = my - (my-view.y)*(k/view.k);
  view.k = k; draw();
}, {passive:false});

document.getElementById("hubs").innerHTML = D.hubs.map(h =>
  '<div class="hub" data-p="'+h.pkg+'"><div class="n">'+h.label+'</div>'+
  '<div class="m">'+h.kind+' · '+h.loc+' · entrada '+h.in+' / saida '+h.out+'</div></div>'
).join("");
document.querySelectorAll(".hub").forEach(el => el.addEventListener("click", () => {
  const p = el.getAttribute("data-p");
  const i = D.nodes.findIndex(n => n.pkg === p);
  sel = i; draw();
}));

const comms = [...new Set(D.nodes.map(n=>n.comm))].sort((a,b)=>a-b).slice(0,10);
document.getElementById("legend").innerHTML = '<h2 style="width:100%">Comunidades</h2>' +
  comms.map(c => '<div><span class="sw" style="background:'+cor(c)+'"></span>C'+c+'</div>').join("");

document.getElementById("q").addEventListener("input", e => {
  filtro = e.target.value.trim().toLowerCase();
  draw();
});
addEventListener("resize", fit);
matchMedia("(prefers-color-scheme: dark)").addEventListener("change", draw);
fit();
</script>
</body>
</html>`

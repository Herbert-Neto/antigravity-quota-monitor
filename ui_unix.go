//go:build !windows

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type uiBucketResponse struct {
	Label     string  `json:"label"`
	Pct       float64 `json:"pct"`
	PctText   string  `json:"pctText"`
	BarColor  string  `json:"barColor"`
	ResetText string  `json:"resetText"`
}

type uiGroupResponse struct {
	Name     string             `json:"name"`
	Subtitle string             `json:"subtitle"`
	Buckets  []uiBucketResponse `json:"buckets"`
}

type uiStateResponse struct {
	AppTitle      string            `json:"appTitle"`
	StatusColor   string            `json:"statusColor"`
	Loading       bool              `json:"loading"`
	LastErr       string            `json:"lastErr"`
	Pinned        bool              `json:"pinned"`
	TitleSummary  string            `json:"titleSummary"`
	FooterText    string            `json:"footerText"`
	FooterColor   string            `json:"footerColor"`
	DesiredHeight int               `json:"desiredHeight"`
	Groups        []uiGroupResponse `json:"groups"`
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(unixCardHTML))
}

func handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSONState(w)
}

func handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	triggerFetch()
	writeJSONState(w)
}

func handlePin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uState.mu.Lock()
	uState.pinned = !uState.pinned
	uState.mu.Unlock()
	writeJSONState(w)
}

func handleQuit(requestQuit func()) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ok":true}`))
		go requestQuit()
	}
}

func writeJSONState(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(buildUIState())
}

func buildUIState() uiStateResponse {
	uState.mu.Lock()
	data := uState.data
	loading := uState.loading
	lastErr := uState.lastErr
	lastUpdated := uState.lastUpdated
	pinned := uState.pinned
	uState.mu.Unlock()

	now := time.Now()
	resp := uiStateResponse{
		AppTitle:      appTitle,
		StatusColor:   statusHexColor(data, loading),
		Loading:       loading,
		LastErr:       lastErr,
		Pinned:        pinned,
		TitleSummary:  buildUnixTitleSummary(data, loading, lastErr),
		DesiredHeight: calcUnixDesiredHeight(data),
		Groups:        make([]uiGroupResponse, 0),
	}

	if data != nil {
		for _, g := range data.Groups {
			ug := uiGroupResponse{
				Name:     g.Name,
				Subtitle: ShortGroupSubtitle(g),
				Buckets:  make([]uiBucketResponse, 0, len(g.Buckets)),
			}
			for _, b := range g.Buckets {
				clamped := clampUnix01(b.RemainingFraction)
				pct := clamped * 100.0
				ug.Buckets = append(ug.Buckets, uiBucketResponse{
					Label:     FriendlyBucketName(b),
					Pct:       pct,
					PctText:   fmt.Sprintf("%.1f%%", pct),
					BarColor:  fractionHexColor(b.RemainingFraction),
					ResetText: FormatCountdown(b.ResetTime, now),
				})
			}
			resp.Groups = append(resp.Groups, ug)
		}
	}

	resp.FooterText = "Aguardando primeira leitura..."
	resp.FooterColor = "#6F7B8F"
	switch {
	case loading:
		resp.FooterText = "Atualizando cotas no agy..."
		resp.FooterColor = "#3B82F6"
	case lastErr != "" && data != nil:
		resp.FooterText = "Aviso na última atualização (exibindo cache)"
		resp.FooterColor = "#F39C12"
	case lastErr != "":
		resp.FooterText = "Falha ao consultar agy"
		resp.FooterColor = "#E74C3C"
	case !lastUpdated.IsZero():
		resp.FooterText = fmt.Sprintf("Atualizado às %s  •  Auto a cada 3m", lastUpdated.Format("15:04:05"))
	}

	return resp
}

func calcUnixDesiredHeight(data *UsageData) int {
	const (
		headerH    = 44
		footerH    = 32
		outerPad   = 12
		groupGap   = 10
		groupPad   = 12
		groupHdrH  = 34
		bucketH    = 44
		emptyBodyH = 110
	)

	logicalH := headerH + outerPad + footerH
	if data == nil || len(data.Groups) == 0 {
		logicalH += emptyBodyH + outerPad
	} else {
		for i, g := range data.Groups {
			if i > 0 {
				logicalH += groupGap
			}
			logicalH += groupPad*2 + groupHdrH + len(g.Buckets)*bucketH
		}
		logicalH += outerPad
	}
	return logicalH
}

func buildUnixTitleSummary(data *UsageData, loading bool, lastErr string) string {
	if data == nil || len(data.Groups) == 0 {
		if loading {
			return "AGY Quota: Atualizando..."
		}
		if lastErr != "" {
			return "AGY Quota: Erro ao consultar"
		}
		return appTitle
	}

	var parts []string
	for _, g := range data.Groups {
		label := g.Name
		switch {
		case strings.Contains(g.Name, "Gemini Models"):
			label = "Gem"
		case strings.Contains(g.Name, "Claude"):
			label = "3P"
		case strings.Contains(g.Name, "Argon"):
			label = "Argon"
		}
		lowest := 1.0
		for _, b := range g.Buckets {
			if b.RemainingFraction < lowest {
				lowest = b.RemainingFraction
			}
		}
		parts = append(parts, fmt.Sprintf("%s: %.0f%%", label, clampUnix01(lowest)*100.0))
	}
	return "AGY Quota | " + strings.Join(parts, " | ")
}

func statusHexColor(data *UsageData, loading bool) string {
	if data == nil {
		if loading {
			return "#3B82F6"
		}
		return "#6F7B8F"
	}
	return fractionHexColor(LowestFraction(data))
}

func fractionHexColor(f float64) string {
	switch {
	case f < 0:
		return "#6F7B8F"
	case f >= 0.50:
		return "#2ECC71"
	case f >= 0.20:
		return "#F39C12"
	default:
		return "#E74C3C"
	}
}

func clampUnix01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

const unixCardHTML = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>AGY Quota Monitor</title>
  <link id="favicon" rel="icon" type="image/png" href="" />
  <style>
    * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
    }

    html, body {
      width: 100%;
      min-height: 100%;
      background: #14171F;
      color: #F3F6FC;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      -webkit-font-smoothing: antialiased;
      user-select: none;
    }

    body {
      display: flex;
      justify-content: center;
      align-items: flex-start;
    }

    .card {
      width: 416px;
      max-width: 100%;
      background: #14171F;
      border: 1px solid #2E3646;
      display: flex;
      flex-direction: column;
      overflow: hidden;
    }

    .header {
      height: 44px;
      background: #1B202B;
      border-bottom: 1px solid #2E3646;
      padding: 0 10px 0 14px;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .status-dot {
      width: 9px;
      height: 9px;
      border-radius: 999px;
      background: #3B82F6;
      flex-shrink: 0;
    }

    .title {
      font-size: 16px;
      font-weight: 600;
      color: #F3F6FC;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      flex: 1;
    }

    .actions {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-shrink: 0;
    }

    button {
      height: 24px;
      border: none;
      border-radius: 6px;
      background: #262D3B;
      color: #F3F6FC;
      font-family: inherit;
      font-size: 12px;
      font-weight: 500;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transition: background 120ms ease;
    }

    button:hover {
      background: #333C4F;
    }

    .btn-pin {
      width: 68px;
    }

    .btn-pin.pinned {
      background: #3B82F6;
    }

    .btn-pin.pinned:hover {
      background: #2563EB;
    }

    .btn-refresh {
      width: 70px;
    }

    .btn-icon {
      width: 26px;
      font-weight: 600;
    }

    .btn-close:hover {
      background: #C0392B;
    }

    .body {
      padding: 12px;
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    .group-card {
      background: #1E2430;
      border: 1px solid #2E3646;
      border-radius: 10px;
      padding: 12px;
    }

    .empty-card {
      height: 110px;
      background: #1E2430;
      border: 1px solid #2E3646;
      border-radius: 10px;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 0 16px;
      text-align: center;
      font-size: 13px;
      color: #9BA6B8;
    }

    .empty-card.error {
      color: #E74C3C;
    }

    .group-header {
      height: 34px;
      display: flex;
      flex-direction: column;
      justify-content: center;
    }

    .group-name {
      font-size: 15px;
      font-weight: 600;
      line-height: 18px;
      color: #F3F6FC;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .group-sub {
      font-size: 12px;
      line-height: 14px;
      color: #9BA6B8;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .bucket {
      height: 44px;
      display: flex;
      flex-direction: column;
      justify-content: center;
    }

    .bucket-top {
      height: 16px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      margin-bottom: 3px;
    }

    .bucket-label {
      font-size: 13px;
      color: #F3F6FC;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .bucket-pct {
      font-size: 13px;
      font-weight: 600;
      flex-shrink: 0;
    }

    .track {
      width: 100%;
      height: 7px;
      border-radius: 6px;
      background: #2A3140;
      overflow: hidden;
    }

    .fill {
      height: 100%;
      border-radius: 6px;
      transition: width 220ms ease;
    }

    .bucket-reset {
      margin-top: 2px;
      height: 15px;
      font-size: 12px;
      line-height: 15px;
      color: #6F7B8F;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .footer {
      height: 32px;
      border-top: 1px solid #2E3646;
      padding: 0 14px;
      display: flex;
      align-items: center;
      font-size: 12px;
      color: #6F7B8F;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
  </style>
</head>
<body>
  <div class="card">
    <header class="header">
      <span id="statusDot" class="status-dot"></span>
      <div class="title">AGY Quota Monitor</div>
      <div class="actions">
        <button id="pinBtn" class="btn-pin pinned" type="button">Fixado</button>
        <button id="refreshBtn" class="btn-refresh" type="button">Atualizar</button>
        <button id="hideBtn" class="btn-icon" type="button" title="Minimizar / Tirar foco">_</button>
        <button id="closeBtn" class="btn-icon btn-close" type="button" title="Sair">X</button>
      </div>
    </header>

    <main id="body" class="body">
      <div class="empty-card">Carregando cotas do agy...</div>
    </main>

    <footer id="footer" class="footer">Aguardando primeira leitura...</footer>
  </div>

  <script>
    const statusDotEl = document.getElementById('statusDot');
    const pinBtnEl = document.getElementById('pinBtn');
    const refreshBtnEl = document.getElementById('refreshBtn');
    const hideBtnEl = document.getElementById('hideBtn');
    const closeBtnEl = document.getElementById('closeBtn');
    const bodyEl = document.getElementById('body');
    const footerEl = document.getElementById('footer');
    const faviconEl = document.getElementById('favicon');

    let lastFaviconColor = '';
    let lastDesiredHeight = 0;
    let loadingTimer = null;

    function updateFavicon(color) {
      if (color === lastFaviconColor) return;
      lastFaviconColor = color;

      const canvas = document.createElement('canvas');
      canvas.width = 32;
      canvas.height = 32;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;

      ctx.clearRect(0, 0, 32, 32);

      // Outer dark ring (#181C26)
      ctx.beginPath();
      ctx.arc(16, 16, 13.5, 0, Math.PI * 2);
      ctx.fillStyle = '#181C26';
      ctx.fill();

      // Inner status dot
      ctx.beginPath();
      ctx.arc(16, 16, 9.8, 0, Math.PI * 2);
      ctx.fillStyle = color;
      ctx.fill();

      faviconEl.href = canvas.toDataURL('image/png');
    }

    function renderState(s) {
      statusDotEl.style.background = s.statusColor;
      updateFavicon(s.statusColor);
      document.title = s.titleSummary || s.appTitle;

      pinBtnEl.textContent = s.pinned ? 'Fixado' : 'Flutuante';
      pinBtnEl.classList.toggle('pinned', !!s.pinned);

      refreshBtnEl.textContent = s.loading ? '...' : 'Atualizar';

      footerEl.textContent = s.footerText || '';
      footerEl.style.color = s.footerColor || '#6F7B8F';

      bodyEl.innerHTML = '';
      if (!s.groups || s.groups.length === 0) {
        const empty = document.createElement('div');
        empty.className = 'empty-card' + (!s.loading && s.lastErr ? ' error' : '');
        empty.textContent = (!s.loading && s.lastErr) ? ('Erro: ' + s.lastErr) : 'Carregando cotas do agy...';
        bodyEl.appendChild(empty);
      } else {
        for (const g of s.groups) {
          const card = document.createElement('section');
          card.className = 'group-card';

          const hdr = document.createElement('div');
          hdr.className = 'group-header';

          const nameEl = document.createElement('div');
          nameEl.className = 'group-name';
          nameEl.textContent = g.name;

          const subEl = document.createElement('div');
          subEl.className = 'group-sub';
          subEl.textContent = g.subtitle;

          hdr.appendChild(nameEl);
          hdr.appendChild(subEl);
          card.appendChild(hdr);

          for (const b of g.buckets) {
            const bucketEl = document.createElement('div');
            bucketEl.className = 'bucket';

            const topEl = document.createElement('div');
            topEl.className = 'bucket-top';

            const labelEl = document.createElement('span');
            labelEl.className = 'bucket-label';
            labelEl.textContent = b.label;

            const pctEl = document.createElement('span');
            pctEl.className = 'bucket-pct';
            pctEl.style.color = b.barColor;
            pctEl.textContent = b.pctText;

            topEl.appendChild(labelEl);
            topEl.appendChild(pctEl);

            const trackEl = document.createElement('div');
            trackEl.className = 'track';

            const fillEl = document.createElement('div');
            fillEl.className = 'fill';
            const fillPct = Math.max(0, Math.min(100, b.pct));
            fillEl.style.width = fillPct + '%';
            if (fillPct > 0) {
              fillEl.style.minWidth = '6px';
            }
            fillEl.style.background = b.barColor;
            trackEl.appendChild(fillEl);

            const resetEl = document.createElement('div');
            resetEl.className = 'bucket-reset';
            resetEl.textContent = b.resetText;

            bucketEl.appendChild(topEl);
            bucketEl.appendChild(trackEl);
            bucketEl.appendChild(resetEl);
            card.appendChild(bucketEl);
          }

          bodyEl.appendChild(card);
        }
      }

      if (s.desiredHeight && s.desiredHeight !== lastDesiredHeight && window.outerWidth <= 460) {
        lastDesiredHeight = s.desiredHeight;
        const chromeH = Math.max(0, window.outerHeight - window.innerHeight);
        try {
          window.resizeTo(416, s.desiredHeight + chromeH);
        } catch (_) {}
      }

      if (s.loading) {
        if (!loadingTimer) {
          loadingTimer = setTimeout(() => {
            loadingTimer = null;
            fetchState();
          }, 800);
        }
      } else if (loadingTimer) {
        clearTimeout(loadingTimer);
        loadingTimer = null;
      }
    }

    async function fetchState() {
      try {
        const res = await fetch('/api/state', { cache: 'no-store' });
        if (!res.ok) return;
        const data = await res.json();
        renderState(data);
      } catch (_) {}
    }

    pinBtnEl.addEventListener('click', async () => {
      try {
        const res = await fetch('/api/pin', { method: 'POST' });
        if (res.ok) renderState(await res.json());
      } catch (_) {}
    });

    refreshBtnEl.addEventListener('click', async () => {
      try {
        const res = await fetch('/api/refresh', { method: 'POST' });
        if (res.ok) renderState(await res.json());
      } catch (_) {}
    });

    hideBtnEl.addEventListener('click', () => {
      window.blur();
    });

    closeBtnEl.addEventListener('click', async () => {
      try {
        await fetch('/api/quit', { method: 'POST' });
      } catch (_) {}
      window.close();
    });

    fetchState();
    setInterval(fetchState, 15000);
  </script>
</body>
</html>
`

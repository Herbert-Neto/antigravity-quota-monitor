# AGY Quota Monitor

[![CI](https://github.com/Herbert-Neto/antigravity-quota-monitor/actions/workflows/ci.yml/badge.svg)](https://github.com/Herbert-Neto/antigravity-quota-monitor/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Herbert-Neto/antigravity-quota-monitor)](https://github.com/Herbert-Neto/antigravity-quota-monitor/releases)
![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Widget leve de monitoramento de cotas dos modelos no **Google Antigravity CLI (`agy`)** para **Windows, Linux e macOS**.

Desenvolvido em **Go puro** com `CGO_ENABLED=0` e **zero dependências externas**:
- **Windows:** usa APIs nativas Win32/GDI (`user32`, `gdi32`, `shell32`, `dwmapi`) com *System Tray* dinâmico e painel flutuante arredondado, sem WebView/Electron.
- **Linux e macOS:** mantém o **mesmo card visual escuro** (`416px`, mesma paleta, barras semânticas e controles) servido localmente em uma janela dedicada (*app mode*), preservando o código nativo do Windows intacto via *build tags* do Go.

## Funcionalidades

- **Visual consistente multiplataforma:** card escuro (`#14171F` / `#1E2430`) com largura de `416px`, ponto de status e barras de progresso coloridas conforme a cota disponível (verde `>= 50%`, amarelo `20%–49%`, vermelho `< 20%`, azul durante carregamento).
- **Ícone / Favicon dinâmico:** no Windows altera o ícone da bandeja do sistema com resumo no *hover*; no Linux/macOS atualiza dinamicamente o favicon e o título da janela.
- **Modos Fixado e Flutuante:**
  - **Fixado:** permanece visível sobre outras janelas (no Windows pode ser arrastado pela barra superior).
  - **Flutuante:** oculta automaticamente para a bandeja no Windows ao clicar fora.
- **Grupos e janelas monitorados:** lê a saída JSON de `agy --print /usage --output-format json` exibindo todos os grupos (`Gemini Models`, `Claude and GPT models`, `Gemini 4 Argon`), barras de progresso (`5h` e `weekly`) e contagem regressiva convertida para o horário local.
- **Atualização eficiente:** consulta o `agy` em segundo plano a cada **3 minutos** (no Windows usando `CREATE_NO_WINDOW` para não piscar terminal) e atualiza as contagens regressivas a cada **15 segundos**.
- **Atalhos da bandeja (Windows):** clique esquerdo alterna a exibição do card; clique direito abre o menu (*Mostrar / Ocultar*, *Modo Fixado*, *Atualizar Agora*, *Sair*).

## Pré-requisitos

- **Windows 10/11** (`amd64`), **Linux** (`amd64`/`arm64`) ou **macOS** (`amd64`/`arm64`)
- **Antigravity CLI (`agy`)** instalado e autenticado (no `PATH`, em `%LOCALAPPDATA%\agy\bin\agy.exe` no Windows, ou em `~/.local/bin/agy`, `~/.agy/bin/agy`, `/opt/homebrew/bin/agy`, `/usr/local/bin/agy` no Linux/macOS)
- **Go 1.26+** (apenas para compilar a partir do código-fonte)

## Como baixar e executar

### Baixar pronto (Windows, Linux e macOS)

Acesse a aba **Releases** do repositório e baixe o binário correspondente ao seu sistema (todos gerados automaticamente via GitHub Actions com checksums em `SHA256SUMS.txt`):

- **Windows (x64):** `agy-quota-windows-amd64.exe`
- **Linux (x64 / ARM64):** `agy-quota-linux-amd64` / `agy-quota-linux-arm64`
- **macOS (Intel / Apple Silicon):** `agy-quota-darwin-amd64` / `agy-quota-darwin-arm64`

#### Executando no Linux ou macOS

```bash
chmod +x ./agy-quota-linux-amd64
./agy-quota-linux-amd64
```

*(No macOS, caso o Gatekeeper bloqueie na primeira execução de binário não assinado: `xattr -d com.apple.quarantine ./agy-quota-darwin-arm64`.)*

### Compilar a partir do código-fonte

#### Windows (GUI sem janela de terminal)

```powershell
go build -ldflags="-H=windowsgui -s -w" -o agy-quota.exe .
```

#### Linux / macOS (nativo)

```bash
go build -ldflags="-s -w" -o agy-quota .
```

#### Cross-compile (gerar binários de outras plataformas)

```powershell
# Windows (a partir de qualquer SO)
$env:CGO_ENABLED="0"; $env:GOOS="windows"; $env:GOARCH="amd64"
go build -ldflags="-H=windowsgui -s -w" -o agy-quota.exe .

# Linux x64
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="amd64"
go build -ldflags="-s -w" -o agy-quota-linux-amd64 .

# Linux ARM64
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="arm64"
go build -ldflags="-s -w" -o agy-quota-linux-arm64 .

# macOS Intel / Apple Silicon
$env:CGO_ENABLED="0"; $env:GOOS="darwin"; $env:GOARCH="amd64"
go build -ldflags="-s -w" -o agy-quota-darwin-amd64 .

$env:CGO_ENABLED="0"; $env:GOOS="darwin"; $env:GOARCH="arm64"
go build -ldflags="-s -w" -o agy-quota-darwin-arm64 .
```

### Modo de teste no terminal (`--test-cli`)

Para testar a coleta e o parser direto no terminal em qualquer sistema operacional:

```powershell
go run . --test-cli
```

### Executar testes unitários

```powershell
go test -v ./...
```

## Iniciar automaticamente com o Windows

1. Pressione `Win + R`, digite `shell:startup` e pressione **Enter**.
2. Crie um **atalho** para o arquivo executável (`agy-quota-windows-amd64.exe` ou `agy-quota.exe`) dentro da pasta aberta.

## Estrutura do projeto

- `go.mod` — definição do módulo Go (sem dependências externas)
- `quota.go` — execução multiplataforma do `agy`, parsing do JSON e formatação de contagens regressivas
- `quota_test.go` — testes unitários multiplataforma do parser e formatação
- `proc_windows.go` — configuração Win32 (`CREATE_NO_WINDOW`) para ocultar janela de console (`//go:build windows`)
- `proc_unix.go` — stub de processo para Linux e macOS (`//go:build !windows`)
- `main.go` — entrada Windows, estado da aplicação, loops de atualização e `wndProc` (`//go:build windows`)
- `ui.go` — cálculo de layout e renderização GDI *double-buffered* no Windows (`//go:build windows`)
- `tray.go` — ícone dinâmico na bandeja do Windows, tooltip e menu de contexto (`//go:build windows`)
- `win32.go` — constantes, declarações de DLLs/structs da API Win32 e utilitários GDI/DPI (`//go:build windows`)
- `main_unix.go` — entrada Linux/macOS, servidor local, loop de atualização e abertura da janela app (`//go:build !windows`)
- `ui_unix.go` — API local e interface HTML5/CSS3/JS do card escuro para Linux e macOS (`//go:build !windows`)

## Licença

Distribuído sob a licença [MIT](LICENSE).

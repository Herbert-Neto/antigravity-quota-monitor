# AGY Quota Monitor

[![CI](https://github.com/Herbert-Neto/antigravity-quota-monitor/actions/workflows/ci.yml/badge.svg)](https://github.com/Herbert-Neto/antigravity-quota-monitor/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Herbert-Neto/antigravity-quota-monitor)](https://github.com/Herbert-Neto/antigravity-quota-monitor/releases)
![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Widget leve para acompanhar as cotas dos modelos no **Google Antigravity CLI (`agy`)** em **Windows, Linux e macOS**.

## Funcionalidades

- **Monitoramento por grupo e janela:** exibe as cotas disponíveis (`5h` e semanal), barras coloridas conforme o saldo e contagem regressiva no horário local.
- **Bandeja e status rápido:** muda de cor conforme a menor cota disponível; no Windows fica na bandeja do sistema com atalho de clique para mostrar/ocultar.
- **Modo Fixado ou Flutuante:** escolha entre manter o card sempre visível ou recolher automaticamente ao clicar fora (no Windows).
- **Atualização automática:** consulta o saldo a cada 3 minutos em segundo plano ou sob demanda pelo botão **Atualizar**.
- **Iniciar com o sistema:** opção integrada para abrir automaticamente no login.
- **Aviso de novas versões:** verifica se há atualizações disponíveis e abre a página de download quando existir uma versão mais recente.

## Pré-requisitos

- **Windows 10/11** (`amd64`), **Linux** (`amd64`/`arm64`) ou **macOS** (`amd64`/`arm64`)
- **Antigravity CLI (`agy`)** instalado e autenticado

## Como baixar e usar

1. Acesse a aba **[Releases](https://github.com/Herbert-Neto/antigravity-quota-monitor/releases)** e baixe o arquivo do seu sistema:
   - **Windows:** `agy-quota-windows-amd64.exe`
   - **Linux:** `agy-quota-linux-amd64` ou `agy-quota-linux-arm64`
   - **macOS:** `agy-quota-darwin-amd64` ou `agy-quota-darwin-arm64`

2. **No Windows:** abra o `.exe` diretamente. Clique com o botão direito no ícone da bandeja para fixar o card, ativar **Iniciar com o Windows**, verificar atualizações ou sair.

3. **No Linux / macOS:** dê permissão de execução e inicie pelo terminal:

```bash
chmod +x ./agy-quota-linux-amd64
./agy-quota-linux-amd64
```

*(No macOS, caso o Gatekeeper bloqueie na primeira vez: `xattr -d com.apple.quarantine ./agy-quota-darwin-arm64`.)*

Para iniciar junto com o sistema no Linux/macOS, basta clicar em **Login: Off / On** no rodapé do card.

## Compilar a partir do código-fonte

Requer **Go 1.26+**:

```powershell
# Windows
go build -ldflags="-H=windowsgui -s -w" -o agy-quota.exe .
```

```bash
# Linux / macOS
go build -ldflags="-s -w" -o agy-quota .
```

## Licença

Distribuído sob a licença [MIT](LICENSE).

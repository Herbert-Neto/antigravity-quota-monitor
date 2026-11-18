# AGY Quota Monitor

Widget leve de bandeja do sistema (*System Tray*) e painel flutuante para Windows que monitora as cotas de uso dos modelos no **Google Antigravity CLI (`agy`)**.

Desenvolvido em **Go puro** utilizando apenas a biblioteca padrão e APIs nativas do Windows (`user32`, `gdi32`, `shell32`, `dwmapi`), sem dependências externas, sem CGO e sem consumo de WebView/Electron.

## Funcionalidades

- **Ícone dinâmico na bandeja do sistema:** muda de cor automaticamente conforme a menor cota disponível (verde `>= 50%`, amarelo `20%–49%`, vermelho `< 20%`, azul durante carregamento) e exibe o resumo no *hover*.
- **Card flutuante escuro (Windows 11):** cantos arredondados via `DwmSetWindowAttribute`, renderização *double-buffered* sem *flicker* e suporte a DPI.
- **Modos Fixado e Flutuante:**
  - **Fixado:** permanece visível sobre outras janelas (pode ser arrastado pela barra superior).
  - **Flutuante:** oculta automaticamente para a bandeja ao clicar fora.
- **Grupos e janelas monitorados:** lê a saída JSON de `agy --print /usage --output-format json` exibindo todos os grupos (`Gemini Models`, `Claude and GPT models`, `Gemini 4 Argon`), barras de progresso para as janelas (`5h` e `weekly`) e contagem regressiva convertida para o horário local.
- **Atualização eficiente:** consulta o `agy` em segundo plano a cada **3 minutos** (sem piscar janela de terminal, usando `CREATE_NO_WINDOW`) e atualiza as contagens regressivas localmente a cada **15 segundos**.
- **Atalhos da bandeja:** clique esquerdo alterna a exibição do card; clique direito abre o menu (*Mostrar / Ocultar*, *Modo Fixado*, *Atualizar Agora*, *Sair*).

## Pré-requisitos

- **Windows 10/11** (x64)
- **Antigravity CLI (`agy`)** instalado e autenticado (no `PATH` ou em `%LOCALAPPDATA%\agy\bin\agy.exe`)
- **Go 1.26+** (apenas para compilar a partir do código-fonte)

## Como compilar

### Executável final (GUI sem janela de terminal)

```powershell
go build -ldflags="-H=windowsgui -s -w" -o agy-quota.exe .
```

### Modo de teste no terminal (`--test-cli`)

Para testar a coleta e o parser direto no terminal (com saída de console):

```powershell
go run . --test-cli
```

Ou compilando sem `-H=windowsgui`:

```powershell
go build -o agy-quota.exe .
.\agy-quota.exe --test-cli
```

## Iniciar automaticamente com o Windows

1. Pressione `Win + R`, digite `shell:startup` e pressione **Enter**.
2. Crie um **atalho** para o arquivo `agy-quota.exe` dentro da pasta aberta.

## Estrutura do projeto

- `go.mod` — definição do módulo Go
- `quota.go` — execução do comando `agy`, parsing do JSON e formatação de contagens regressivas
- `main.go` — janela Win32, renderização GDI, ícone da bandeja e modo `--test-cli`

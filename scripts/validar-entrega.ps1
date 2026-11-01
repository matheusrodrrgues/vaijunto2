$ErrorActionPreference = "Stop"
$raizProjeto = Split-Path -Parent $PSScriptRoot
Set-Location $raizProjeto

function Executar-Etapa {
    param([string]$Nome, [scriptblock]$Comando)
    Write-Host ""
    Write-Host "=== $Nome ==="
    & $Comando
    if ($LASTEXITCODE -ne 0) { throw "Falha na etapa: $Nome" }
}

Write-Host ""
Write-Host "=== Formatação dos componentes novos ==="
# O checkout no Windows mantém parte do legado em CRLF; conferir todo o projeto
# faria o gofmt apontar somente a diferença de quebra de linha.
$caminhosGo = Get-ChildItem -Path preparador,simulador -Recurse -Filter "*.go" |
    Select-Object -ExpandProperty FullName
$naoFormatados = & gofmt -l $caminhosGo
if ($LASTEXITCODE -ne 0) { throw "Não foi possível conferir a formatação Go." }
if ($naoFormatados) { throw "Arquivos sem gofmt: $($naoFormatados -join ', ')" }
Executar-Etapa "Análise estática" { go vet ./... }
Executar-Etapa "Testes automatizados" { go test -count=1 -v ./... }
Executar-Etapa "Configuração Docker Compose" { docker compose config --quiet }

Write-Host ""
Write-Host "Validação local concluída sem erros."
Write-Host "Ainda é obrigatório executar o roteiro de laboratório em computadores distintos."

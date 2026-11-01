param(
    [string]$Data = "2026-11-05",
    [int]$Requisicoes = 60,
    [int]$Concorrencia = 60,
    [switch]$SemBuild
)

$ErrorActionPreference = "Stop"
$raizProjeto = Split-Path -Parent $PSScriptRoot
Set-Location $raizProjeto

function Conferir-Saida {
    param([string]$Etapa)
    if ($LASTEXITCODE -ne 0) { throw "Falha na etapa: $Etapa" }
}

Write-Host "=== 1. Preparação das três empresas ==="
if ($SemBuild) {
    & "$PSScriptRoot\preparar-demonstracao.ps1" -Data $Data -SemBuild
} else {
    & "$PSScriptRoot\preparar-demonstracao.ps1" -Data $Data
}
if (-not $?) { throw "Não foi possível preparar a demonstração." }

if (-not $SemBuild) {
    Write-Host ""
    Write-Host "=== 2. Construção do simulador ==="
    docker compose --profile teste build simulador
    Conferir-Saida "construção do simulador"
}

Write-Host ""
Write-Host "=== 3. Disputa simultânea do último assento ==="
docker compose --profile teste run --rm simulador `
    -data $Data `
    -requisicoes $Requisicoes `
    -concorrencia $Concorrencia `
    -capacidade 1 `
    -espera-recuperacao 17s `
    -json relatorios/concorrencia.json `
    -csv relatorios/concorrencia.csv
Conferir-Saida "teste de concorrência"

Write-Host ""
Write-Host "=== 4. Novo conjunto de assentos para o teste de falha ==="
docker compose --profile demonstracao run --rm preparador -data $Data -assentos 1
Conferir-Saida "preparação do teste de falha"

Write-Host ""
Write-Host "=== 5. Queda e recuperação da LulaTransportes ==="
go run ./simulador `
    -data $Data `
    -duracao 30s `
    -concorrencia 20 `
    -capacidade 1 `
    -falha-servico empresa-b `
    -falha-apos 5s `
    -falha-duracao 5s `
    -espera-recuperacao 17s `
    -json relatorios/falha-recuperacao.json `
    -csv relatorios/falha-recuperacao.csv
Conferir-Saida "teste de falha e recuperação"

Write-Host ""
Write-Host "=== 6. Diagnóstico final ==="
$diagnosticos = @(
    "http://127.0.0.1:8081/api/v1/diagnostics/transactions",
    "http://127.0.0.1:8082/api/v1/diagnostics/transactions",
    "http://127.0.0.1:8083/api/v1/diagnostics/transactions"
)
foreach ($endereco in $diagnosticos) {
    $resultado = Invoke-RestMethod -Uri $endereco -TimeoutSec 3
    $resultado | Format-List empresa_id, preparadas, confirmadas, abortadas, coordenacoes_pendentes, coordenacoes_concluidas, coordenacoes_abortadas
    if ($resultado.preparadas -ne 0 -or $resultado.coordenacoes_pendentes -ne 0) {
        throw "O diagnóstico de $($resultado.empresa_id) ainda possui trabalho pendente."
    }
}

Write-Host ""
Write-Host "TODOS OS TESTES FORAM APROVADOS."
Write-Host "Relatórios:"
Write-Host "- relatorios/concorrencia.json"
Write-Host "- relatorios/falha-recuperacao.json"

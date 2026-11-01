param(
    [string]$Data = "2026-11-05",
    [switch]$SemBuild
)

$ErrorActionPreference = "Stop"
$raizProjeto = Split-Path -Parent $PSScriptRoot
Set-Location $raizProjeto

Write-Host "[1/4] Conferindo o Docker Engine..."
docker version | Out-Null
if ($LASTEXITCODE -ne 0) { throw "O Docker Engine não está disponível." }

if (-not $SemBuild) {
    Write-Host "[2/4] Construindo as imagens..."
    docker compose --profile demonstracao build
    if ($LASTEXITCODE -ne 0) { throw "A construção das imagens falhou." }
} else {
    Write-Host "[2/4] Usando as imagens já construídas."
}

Write-Host "[3/4] Iniciando as três empresas..."
docker compose up -d empresa-a empresa-b empresa-c
if ($LASTEXITCODE -ne 0) { throw "Não foi possível iniciar as empresas." }

$enderecos = @(
    "http://127.0.0.1:8081/api/v1/health",
    "http://127.0.0.1:8082/api/v1/health",
    "http://127.0.0.1:8083/api/v1/health"
)
foreach ($endereco in $enderecos) {
    $pronto = $false
    foreach ($tentativa in 1..30) {
        try {
            $resposta = Invoke-RestMethod -Uri $endereco -TimeoutSec 2
            if ($resposta.status -eq "online") { $pronto = $true; break }
        } catch {
            Start-Sleep -Seconds 1
        }
    }
    if (-not $pronto) { throw "Servidor não ficou disponível: $endereco" }
}

Write-Host "[4/4] Publicando o itinerário distribuído..."
docker compose --profile demonstracao run --rm preparador -data $Data
if ($LASTEXITCODE -ne 0) { throw "O cenário não pôde ser preparado." }

Write-Host ""
Write-Host "Demonstração pronta para $Data."
Write-Host "Consórcio: http://127.0.0.1:8081/api/v1/consortium/health"

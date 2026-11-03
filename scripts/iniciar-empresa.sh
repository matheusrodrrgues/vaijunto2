#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "Uso: bash scripts/iniciar-empresa.sh <empresa-a|empresa-b|empresa-c> <IP_A> <IP_B> <IP_C>" >&2
  exit 1
fi

empresa="$1"
ip_a="$2"
ip_b="$3"
ip_c="$4"

case "$empresa" in
  empresa-a)
    nome="CaronasGo"
    pares="empresa-b=http://${ip_b}:8080,empresa-c=http://${ip_c}:8080"
    ;;
  empresa-b)
    nome="LulaTransportes"
    pares="empresa-a=http://${ip_a}:8080,empresa-c=http://${ip_c}:8080"
    ;;
  empresa-c)
    nome="TetraCaronas"
    pares="empresa-a=http://${ip_a}:8080,empresa-b=http://${ip_b}:8080"
    ;;
  *)
    echo "Empresa inválida: $empresa" >&2
    exit 1
    ;;
esac

raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$raiz/dados/$empresa" "$raiz/logs/$empresa"

if docker container inspect "$empresa" >/dev/null 2>&1; then
  docker start "$empresa" >/dev/null
  echo "$empresa iniciada. Para recriar a configuração: docker rm -f $empresa"
  exit 0
fi

docker run -d \
  --name "$empresa" \
  --restart unless-stopped \
  -p 9000:9000 \
  -p 8080:8080 \
  -e "VAIJUNTO_EMPRESA_ID=$empresa" \
  -e "VAIJUNTO_EMPRESA_NOME=$nome" \
  -e VAIJUNTO_ADDR=:9000 \
  -e VAIJUNTO_REST_ADDR=:8080 \
  -e VAIJUNTO_REST_TIMEOUT=2s \
  -e "VAIJUNTO_PARES=$pares" \
  -e VAIJUNTO_DADOS_DIR=/app/dados \
  -e VAIJUNTO_LOGS_DIR=/app/logs \
  -v "$raiz/dados/$empresa:/app/dados" \
  -v "$raiz/logs/$empresa:/app/logs" \
  vaijunto-servidor

echo "$empresa disponível em TCP :9000 e REST :8080"

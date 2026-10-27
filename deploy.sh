#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

if [ ! -f .env ]; then
  echo "Xatolik: .env fayl mavjud emas!"
  echo "Quyidagi buyruq orqali .env faylni yarating va ichidagi parollarni to'ldiring:"
  echo "  cp .env.example .env"
  exit 1
fi

PROFILE_FLAG=""
if [ "$1" == "--ssl" ]; then
  PROFILE_FLAG="--profile ssl"
fi

echo "==> Docker imijlarini qurish va yangilash..."
docker compose $PROFILE_FLAG build
docker compose $PROFILE_FLAG up -d --remove-orphans

echo "==> Konteynerlar holati:"
docker compose $PROFILE_FLAG ps

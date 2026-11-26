# Host-side Tuna runner for Windows.
# Tuna must be logged in on this machine; Docker cannot copy a browser session.
param(
    [string]$Domain = $env:TUNA_DOMAIN,
    [int]$Port = 8080
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($Domain)) {
    throw 'Укажите домен: .\scripts\tuna-host.ps1 -Domain medvedmed.ru'
}

if (-not (Get-Command tuna -ErrorAction SilentlyContinue)) {
    Write-Host 'Tuna CLI не найдена, устанавливаю через winget...'
    winget install --id yuccastream.tuna -e --accept-package-agreements --accept-source-agreements
    $env:Path = "$env:LOCALAPPDATA\Microsoft\WindowsApps;$env:Path"
}

if (-not (Get-Command tuna -ErrorAction SilentlyContinue)) {
    throw 'Tuna установлена, но tuna.exe не найдена в PATH. Перезапустите PowerShell.'
}

& tuna whoami 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host 'Вход в Tuna: откроется браузер, подтвердите вход в свой аккаунт.'
    & tuna login
}

Write-Host "Запускаю туннель http://127.0.0.1:$Port -> https://$Domain"
& tuna http $Port --domain=$Domain

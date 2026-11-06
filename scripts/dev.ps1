# Runs the backend on this computer: starts Postgres in Docker, applies the
# migrations, then runs the API and the worker with their logs in this terminal.
# Ctrl+C stops the API and the worker. Postgres keeps running, stop it with:
#   docker compose -f deploy/compose.yaml stop
#
# Needs Docker Desktop running, and DATABASE_URL in server/.env (see deploy/ARCHITECTURE.md).

$env:Path = [Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + [Environment]::GetEnvironmentVariable('Path', 'User')

$root = Split-Path -Parent $PSScriptRoot
$server = Join-Path $root 'server'
$compose = Join-Path $root 'deploy\compose.yaml'
$bin = Join-Path $env:TEMP 'warden-dev'

function Stop-Here($message) {
    Write-Host "error: $message" -ForegroundColor Red
    exit 1
}

docker info 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) { Stop-Here 'Docker is not running. Start Docker Desktop, then run this again.' }

docker compose -f $compose up -d --wait
if ($LASTEXITCODE -ne 0) { Stop-Here 'Postgres did not start.' }

Push-Location $server
try {
    go run ./cmd/migrate
    if ($LASTEXITCODE -ne 0) { Stop-Here 'The migrations did not apply.' }

    # Built first, so Ctrl+C reaches the programs themselves and not "go run".
    go build -o "$bin/" ./cmd/api ./cmd/worker
    if ($LASTEXITCODE -ne 0) { Stop-Here 'The build failed.' }

    $api = Start-Process -FilePath "$bin\api.exe" -WorkingDirectory $server -NoNewWindow -PassThru
    $worker = Start-Process -FilePath "$bin\worker.exe" -WorkingDirectory $server -NoNewWindow -PassThru
    try {
        while (-not $api.HasExited -and -not $worker.HasExited) { Start-Sleep -Milliseconds 500 }
        Write-Host 'One program stopped, stopping the other.' -ForegroundColor Yellow
    } finally {
        # Ctrl+C reaches both programs too, so this is usually only a wait.
        foreach ($program in @($api, $worker)) {
            if ($program -and -not $program.WaitForExit(15000)) { $program.Kill() }
        }
    }
} finally {
    Pop-Location
}

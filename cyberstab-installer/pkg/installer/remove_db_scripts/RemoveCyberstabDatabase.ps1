# Removes okidoci_db and Cyberstab roles (okidoci_*, oki_*). Does not uninstall PostgreSQL.
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File .\RemoveCyberstabDatabase.ps1
param(
    [string]$PgHost = "127.0.0.1",
    [int]$PgPort = 5432,
    [string]$PostgresPassword = ""
)

$ErrorActionPreference = "Stop"

function Find-PsqlExe {
    $globs = @(
        (Join-Path $env:ProgramFiles "PostgreSQL\*\bin\psql.exe")
    )
    if ($env:ProgramFiles(x86)) {
        $globs += (Join-Path ${env:ProgramFiles(x86)} "PostgreSQL\*\bin\psql.exe")
    }
    foreach ($g in $globs) {
        $hit = Get-Item $g -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1
        if ($hit) { return $hit.FullName }
    }
    $cmd = Get-Command psql.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    throw "psql.exe not found. Install PostgreSQL or add psql to PATH."
}

function Read-PostgresPassword {
    if (-not [string]::IsNullOrWhiteSpace($PostgresPassword)) { return $PostgresPassword }
    $sec = Read-Host "Postgres password (user postgres)" -AsSecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec)
    try {
        return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)
    } finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
    }
}

$psql = Find-PsqlExe
$env:PGPASSWORD = Read-PostgresPassword

Write-Host "Using: $psql"
Write-Host "Host: $PgHost port: $PgPort"
Write-Host "Dropping okidoci_db and Cyberstab roles (PostgreSQL service is not removed)..."

$delSec = "DO `$\$` BEGIN IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='sec_user') THEN DELETE FROM public.sec_user; END IF; END `$\$`;"
& $psql -U postgres -h $PgHost -p $PgPort -d okidoci_db -v ON_ERROR_STOP=0 -c $delSec 2>$null

& $psql -U postgres -h $PgHost -p $PgPort -d postgres -v ON_ERROR_STOP=0 -c "ALTER DATABASE okidoci_db CONNECTION LIMIT 0;" 2>$null
& $psql -U postgres -h $PgHost -p $PgPort -d postgres -v ON_ERROR_STOP=0 -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='okidoci_db' AND pid <> pg_backend_pid();" 2>$null
Start-Sleep -Milliseconds 500

& $psql -U postgres -h $PgHost -p $PgPort -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS okidoci_db;"
if ($LASTEXITCODE -ne 0) { throw "DROP DATABASE failed (exit $LASTEXITCODE)" }

$dropOki = @"
DO `$`$ DECLARE r record;
BEGIN
  FOR r IN
    SELECT rolname FROM pg_roles
    WHERE rolname LIKE 'oki\_%' ESCAPE '\'
      AND rolname NOT IN ('okidoci_admin','okidoci_service_user_name','okidoci_users')
  LOOP
    EXECUTE format('DROP ROLE %I', r.rolname);
  END LOOP;
END `$`$;
"@
& $psql -U postgres -h $PgHost -p $PgPort -d postgres -v ON_ERROR_STOP=1 -c $dropOki

$dropFixed = @"
DO `$`$ BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'okidoci_admin') THEN EXECUTE 'DROP ROLE okidoci_admin'; END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'okidoci_service_user_name') THEN EXECUTE 'DROP ROLE okidoci_service_user_name'; END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'okidoci_users') THEN EXECUTE 'DROP ROLE okidoci_users'; END IF;
END `$`$;
"@
& $psql -U postgres -h $PgHost -p $PgPort -d postgres -v ON_ERROR_STOP=1 -c $dropFixed

Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
Write-Host "Done. Database and Cyberstab roles removed."

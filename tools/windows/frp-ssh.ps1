param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$Target,

  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$SshArgs
)

$ErrorActionPreference = 'Stop'

$PublicHost = '140.143.209.222'
$AuthUrl = "http://$PublicHost`:7500/api/user/activate"
$TokenPath = Join-Path $env:USERPROFILE '.frp-ssh\token.xml'
$IdentityFile = Join-Path $env:USERPROFILE '.ssh\id_ed25519'

$Services = @{
  '114'      = @{ Port = 6222; TargetIP = '210.47.163.114'; User = 'root' }
  '113'      = @{ Port = 6223; TargetIP = '210.47.163.113'; User = 'root' }
  '118'      = @{ Port = 6224; TargetIP = '210.47.163.118'; User = 'root' }
  '181'      = @{ Port = 6225; TargetIP = '210.47.163.181'; User = 'root' }
  '3'        = @{ Port = 6226; TargetIP = '10.2.0.3'; User = 'root' }
  '003'      = @{ Port = 6226; TargetIP = '10.2.0.3'; User = 'root' }
  '10.2.0.3' = @{ Port = 6226; TargetIP = '10.2.0.3'; User = 'root' }
  '102'      = @{ Port = 6227; TargetIP = '10.2.0.102'; User = 'root' }
  '10.2.0.102' = @{ Port = 6227; TargetIP = '10.2.0.102'; User = 'root' }
}

$DisplayTargets = @('114', '113', '118', '181', '3', '102')

function Get-PlainToken {
  if (-not (Test-Path -LiteralPath $TokenPath)) {
    throw "Missing token file: $TokenPath. Recreate it with: ConvertTo-SecureString '<token>' -AsPlainText -Force | Export-Clixml $TokenPath"
  }

  $secure = Import-Clixml -LiteralPath $TokenPath
  $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
  try {
    [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
  } finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
  }
}

function Show-Targets {
  foreach ($name in $DisplayTargets) {
    $svc = $Services[$name]
    "{0,-9} -> {1}:{2} ({3}@{4})" -f $name, $PublicHost, $svc.Port, $svc.User, $svc.TargetIP
  }
}

if ($Target -in @('list', 'ls', '--list', '-l')) {
  Show-Targets
  exit 0
}

if (-not $Services.ContainsKey($Target)) {
  Write-Error "Unknown target '$Target'. Available targets:"
  Show-Targets | ForEach-Object { Write-Error "  $_" }
  exit 2
}

$svc = $Services[$Target]
$token = Get-PlainToken
$body = @{ token = $token; port = $svc.Port } | ConvertTo-Json -Compress

try {
  Invoke-RestMethod -Uri $AuthUrl -Method POST -ContentType 'application/json' -Body $body | Out-Null
} catch {
  throw "FRP activate failed for port $($svc.Port): $($_.Exception.Message)"
}

$sshCommand = @(
  '-o', 'StrictHostKeyChecking=accept-new',
  '-o', 'ServerAliveInterval=30',
  '-o', 'ServerAliveCountMax=3',
  '-i', $IdentityFile,
  '-p', [string]$svc.Port,
  "$($svc.User)@$PublicHost"
)

if ($SshArgs) {
  $sshCommand += $SshArgs
}

& ssh @sshCommand
exit $LASTEXITCODE

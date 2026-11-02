# Extracted from the reviewed Stonehenge directory playbook.
param(
    [string]$ShareName,
    [string]$ShareReader,
    [string]$ShareFile,
    [string]$EngineeringUser,
    [string]$EngineeringPassword
)

$sharePath = 'C:\ProgramData\Stonehenge\engineering-access'
$publicNote = 'C:\Users\Public\Documents\ENG-1847.txt'
$shareFilePath = Join-Path $sharePath $ShareFile
$changed = $false
foreach ($path in @($sharePath, (Split-Path $publicNote -Parent))) {
    if (-not (Test-Path -LiteralPath $path -PathType Container)) {
        New-Item -ItemType Directory -Path $path -Force | Out-Null
        $changed = $true
    }
}

$note = @'
FCU SUPPORT TICKET ENG-1847
System: vendor.fcu.gov legacy shipment worker

The compatibility worker still accepts the former Java object-stream
export format. The runtime support inventory identifies the pinned
compatibility family. Review the application's diagnostic mappings and
information before preparing a controlled replay.
'@
$handoff = @"
FCU Engineering access handoff
Ticket: ENG-1847
Username: $EngineeringUser@fcu.gov
Password: $EngineeringPassword
Access: Engineering User
"@
foreach ($item in @(
    @{Path = $publicNote; Content = $note},
    @{Path = $shareFilePath; Content = $handoff}
)) {
    $current = if (Test-Path -LiteralPath $item.Path -PathType Leaf) {
        Get-Content -LiteralPath $item.Path -Raw -ErrorAction Stop
    } else { $null }
    if ($current -cne $item.Content) {
        Set-Content -LiteralPath $item.Path -Value $item.Content `
            -Encoding UTF8 -NoNewline -ErrorAction Stop
        $changed = $true
    }
}

$reader = Get-ADUser -Identity $ShareReader -ErrorAction Stop
$readerSid = $reader.SID
$systemSid = [Security.Principal.SecurityIdentifier]'S-1-5-18'
$adminsSid = [Security.Principal.SecurityIdentifier]'S-1-5-32-544'
$inheritance = [Security.AccessControl.InheritanceFlags]'ContainerInherit, ObjectInherit'
$none = [Security.AccessControl.PropagationFlags]::None
$allow = [Security.AccessControl.AccessControlType]::Allow
$acl = [Security.AccessControl.DirectorySecurity]::new()
$acl.SetAccessRuleProtection($true, $false)
$acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new(
    $systemSid, 'FullControl', $inheritance, $none, $allow
))
$acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new(
    $adminsSid, 'FullControl', $inheritance, $none, $allow
))
$acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new(
    $readerSid, 'ReadAndExecute', $inheritance, $none, $allow
))
Set-Acl -LiteralPath $sharePath -AclObject $acl -ErrorAction Stop

$share = Get-SmbShare -Name $ShareName -ErrorAction SilentlyContinue
if ($null -eq $share) {
    New-SmbShare -Name $ShareName -Path $sharePath `
        -FullAccess 'BUILTIN\Administrators' `
        -ReadAccess "FCU\$ShareReader" `
        -FolderEnumerationMode AccessBased -EncryptData $true `
        -ErrorAction Stop | Out-Null
    $changed = $true
}
elseif ($share.Path -ine $sharePath) {
    throw "The managed share name already points at another path."
}
else {
    Set-SmbShare -Name $ShareName -FolderEnumerationMode AccessBased `
        -EncryptData $true -Force -ErrorAction Stop | Out-Null
}

$desired = @{}
$desired[$adminsSid.Value] = 'Full'
$desired[$readerSid.Value] = 'Read'
foreach ($ace in @(Get-SmbShareAccess -Name $ShareName -ErrorAction Stop)) {
    try {
        $sid = ([Security.Principal.NTAccount]$ace.AccountName).Translate(
            [Security.Principal.SecurityIdentifier]
        ).Value
    }
    catch { $sid = '' }
    if (-not $desired.ContainsKey($sid) -or $desired[$sid] -cne [string]$ace.AccessRight) {
        Revoke-SmbShareAccess -Name $ShareName `
            -AccountName $ace.AccountName -Force -ErrorAction Stop
        $changed = $true
    }
}
$access = @(Get-SmbShareAccess -Name $ShareName -ErrorAction Stop)
foreach ($entry in @(
    @{Account = 'BUILTIN\Administrators'; Right = 'Full'},
    @{Account = "FCU\$ShareReader"; Right = 'Read'}
)) {
    if (-not ($access | Where-Object {
        $_.AccountName -ieq $entry.Account -and
        [string]$_.AccessRight -ceq $entry.Right
    })) {
        Grant-SmbShareAccess -Name $ShareName `
            -AccountName $entry.Account -AccessRight $entry.Right `
            -Force -ErrorAction Stop | Out-Null
        $changed = $true
    }
}

$Ansible.Changed = $changed
$Ansible.Result = [ordered]@{
    Share = $ShareName
    Breadcrumb = 'ENG-1847'
    Reader = $ShareReader
}

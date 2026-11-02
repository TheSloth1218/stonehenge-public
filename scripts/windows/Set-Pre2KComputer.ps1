# Extracted from the reviewed Stonehenge directory playbook.
param(
    [string]$ComputerName,
    [string]$ResetTarget,
    [string]$StaffUsersJson
)

Import-Module ActiveDirectory -ErrorAction Stop
$sam = "$ComputerName`$"
$retiredSam = 'FCU-LEGACY$'
$stagingName = 'SH-PRE2K-TMP'
$stagingSam = "$stagingName`$"
$weakPassword = $ComputerName.ToLowerInvariant()
$securePassword = ConvertTo-SecureString $weakPassword `
    -AsPlainText -Force
$description = 'Pre-created legacy workstation account'
$computer = Get-ADComputer -Filter "SamAccountName -eq '$sam'" `
    -Properties Description, Enabled, Name, PasswordNeverExpires, `
        PasswordNotRequired, pwdLastSet -ErrorAction Stop
$retiredComputer = $null
if ($sam -cne $retiredSam) {
    $retiredComputer = Get-ADComputer `
        -Filter "SamAccountName -eq '$retiredSam'" `
        -Properties Enabled -ErrorAction Stop
}
$stagingComputer = Get-ADComputer `
    -Filter "SamAccountName -eq '$stagingSam'" `
    -Properties Description, pwdLastSet -ErrorAction Stop
$managedComputerSids = @()
foreach ($candidate in @(
    $computer, $retiredComputer, $stagingComputer
)) {
    if ($null -ne $candidate) {
        $managedComputerSids += [string]$candidate.SID
    }
}
$changed = $false
if (
    $null -ne $retiredComputer -and
    $retiredComputer.Enabled
) {
    Disable-ADAccount -Identity $retiredComputer -ErrorAction Stop
    $retiredComputer.Enabled = $false
    $changed = $true
}
$needsRecreation = (
    $null -eq $computer -or
    [int64]$computer.pwdLastSet -eq 0 -or
    $computer.Description -cne $description
)
if ($needsRecreation) {
    if ($null -ne $computer) {
        Remove-ADComputer -Identity $computer -Confirm:$false `
            -ErrorAction Stop
    }
    if (
        $null -ne $stagingComputer -and
        (
            [int64]$stagingComputer.pwdLastSet -eq 0 -or
            $stagingComputer.Description -cne `
                'Stonehenge pre2k staging account'
        )
    ) {
        Remove-ADComputer -Identity $stagingComputer -Confirm:$false `
            -ErrorAction Stop
        $stagingComputer = $null
    }
    if ($null -eq $stagingComputer) {
        New-ADComputer -Name $stagingName `
            -SamAccountName $stagingSam `
            -AccountPassword $securePassword -Enabled $true `
            -PasswordNeverExpires $true `
            -Description 'Stonehenge pre2k staging account' `
            -Path 'CN=Computers,DC=fcu,DC=gov' -ErrorAction Stop
        $stagingComputer = Get-ADComputer -Identity $stagingSam `
            -Properties Description, pwdLastSet -ErrorAction Stop
        $managedComputerSids += [string]$stagingComputer.SID
    }
    Set-ADComputer -Identity $stagingComputer `
        -SamAccountName $sam -ErrorAction Stop
    $computer = Get-ADComputer -Identity $sam `
        -Properties Description, Enabled, Name, PasswordNeverExpires, `
            PasswordNotRequired, pwdLastSet -ErrorAction Stop
    $changed = $true
}
elseif ($null -ne $stagingComputer) {
    Remove-ADComputer -Identity $stagingComputer -Confirm:$false `
        -ErrorAction Stop
    $changed = $true
}
if ($computer.Name -cne $ComputerName) {
    Rename-ADObject -Identity $computer.DistinguishedName `
        -NewName $ComputerName -ErrorAction Stop
    $changed = $true
}
if (
    $computer.Description -cne $description -or
    -not $computer.Enabled -or
    -not $computer.PasswordNeverExpires -or
    -not $computer.PasswordNotRequired
) {
    $changed = $true
}
Set-ADComputer -Identity $sam `
    -Description $description `
    -ErrorAction Stop
Set-ADAccountControl -Identity $sam -Enabled $true `
    -PasswordNeverExpires $true -PasswordNotRequired $true `
    -ErrorAction Stop

$computer = Get-ADComputer -Identity $sam `
    -Properties Enabled, PasswordNeverExpires, PasswordNotRequired, `
        pwdLastSet -ErrorAction Stop
if (
    -not $computer.Enabled -or
    -not $computer.PasswordNeverExpires -or
    -not $computer.PasswordNotRequired -or
    [int64]$computer.pwdLastSet -eq 0
) {
    throw 'The pre-created computer flags are invalid.'
}
$computerSid = $computer.SID
if ($managedComputerSids -notcontains [string]$computerSid) {
    $managedComputerSids += [string]$computerSid
}
$resetPasswordGuid = [Guid]'00299570-246d-11d0-a768-00aa006e0529'
$emptyGuid = [Guid]::Empty
$staffUsers = $StaffUsersJson | ConvertFrom-Json -ErrorAction Stop

foreach ($staff in $staffUsers) {
    $user = Get-ADUser -Identity $staff.username -ErrorAction Stop
    $aclPath = "AD:\$($user.DistinguishedName)"
    $acl = Get-Acl -Path $aclPath -ErrorAction Stop
    $computerRules = @($acl.Access | Where-Object {
        try {
            $sid = $_.IdentityReference.Translate(
                [Security.Principal.SecurityIdentifier]
            )
        }
        catch { return $false }
        ($managedComputerSids -contains [string]$sid) -and
            -not $_.IsInherited
    })
    $exactRuleSid = $null
    if ($computerRules.Count -eq 1) {
        try {
            $exactRuleSid = $computerRules[0].IdentityReference.Translate(
                [Security.Principal.SecurityIdentifier]
            )
        }
        catch { $exactRuleSid = $null }
    }
    $hasExactReset = (
        $computerRules.Count -eq 1 -and
        $exactRuleSid -eq $computerSid -and
        $computerRules[0].AccessControlType -eq `
            [Security.AccessControl.AccessControlType]::Allow -and
        $computerRules[0].ActiveDirectoryRights -eq `
            [DirectoryServices.ActiveDirectoryRights]::ExtendedRight -and
        $computerRules[0].ObjectType -eq $resetPasswordGuid
    )
    if (
        ($staff.username -ieq $ResetTarget -and $hasExactReset) -or
        ($staff.username -ine $ResetTarget -and $computerRules.Count -eq 0)
    ) {
        continue
    }
    foreach ($ace in $computerRules) {
        $acl.RemoveAccessRuleSpecific($ace)
    }
    if ($staff.username -ieq $ResetTarget) {
        $acl.AddAccessRule(
            [DirectoryServices.ActiveDirectoryAccessRule]::new(
                $computerSid,
                [DirectoryServices.ActiveDirectoryRights]::ExtendedRight,
                [Security.AccessControl.AccessControlType]::Allow,
                $resetPasswordGuid,
                [DirectoryServices.ActiveDirectorySecurityInheritance]::None,
                $emptyGuid
            )
        )
    }
    Set-Acl -Path $aclPath -AclObject $acl -ErrorAction Stop
    $changed = $true
}

$Ansible.Changed = $changed
$Ansible.Result = [ordered]@{
    Computer = $sam
    ResetTarget = $ResetTarget
}

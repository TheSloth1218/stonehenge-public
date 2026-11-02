# Extracted from the reviewed Stonehenge directory playbook.
param(
    [string]$ComputerName,
    [string]$DelegationUser,
    [string]$DelegationSpn
)

Import-Module ActiveDirectory -ErrorAction Stop
$computerSam = "$ComputerName`$"
$changed = $false
$computer = Get-ADComputer -Identity $computerSam `
    -Properties msDS-AllowedToDelegateTo, ServicePrincipalName, `
        TrustedForDelegation, TrustedToAuthForDelegation `
    -ErrorAction Stop
$requiredSpns = @(
    "HOST/$ComputerName",
    "HOST/$ComputerName.fcu.gov"
)
$missingSpns = @($requiredSpns | Where-Object {
    @($computer.ServicePrincipalName) -notcontains $_
})
if ($missingSpns.Count -gt 0) {
    Set-ADComputer -Identity $computer `
        -Add @{servicePrincipalName = [string[]]$missingSpns} `
        -ErrorAction Stop
    $changed = $true
}

$currentDelegation = @(
    $computer.'msDS-AllowedToDelegateTo' | Sort-Object
) -join "`0"
if ($currentDelegation -cne $DelegationSpn) {
    Set-ADObject -Identity $computer.DistinguishedName `
        -Replace @{'msDS-AllowedToDelegateTo' = $DelegationSpn} `
        -ErrorAction Stop
    $changed = $true
}
if (
    -not $computer.TrustedToAuthForDelegation -or
    $computer.TrustedForDelegation
) {
    Set-ADAccountControl -Identity $computer `
        -TrustedToAuthForDelegation $true `
        -TrustedForDelegation $false -ErrorAction Stop
    $changed = $true
}

$domainAdmins = @(Get-ADGroupMember -Identity 'Domain Admins' `
    -Recursive -ErrorAction Stop | Where-Object objectClass -eq 'user')
foreach ($member in $domainAdmins) {
    $user = Get-ADUser -Identity $member `
        -Properties AccountNotDelegated -ErrorAction Stop
    $mustBeSensitive = $user.SamAccountName -ine $DelegationUser
    if ($user.AccountNotDelegated -ne $mustBeSensitive) {
        Set-ADAccountControl -Identity $user `
            -AccountNotDelegated $mustBeSensitive -ErrorAction Stop
        $changed = $true
    }
}

$computer = Get-ADComputer -Identity $computerSam `
    -Properties msDS-AllowedToDelegateTo, ServicePrincipalName, `
        TrustedForDelegation, TrustedToAuthForDelegation `
    -ErrorAction Stop
$delegableAdmins = @($domainAdmins | Where-Object {
    -not (Get-ADUser -Identity $_ `
        -Properties AccountNotDelegated).AccountNotDelegated
})
if (
    @($computer.'msDS-AllowedToDelegateTo').Count -ne 1 -or
    $computer.'msDS-AllowedToDelegateTo'[0] -cne $DelegationSpn -or
    -not $computer.TrustedToAuthForDelegation -or
    $computer.TrustedForDelegation -or
    @($requiredSpns | Where-Object {
        @($computer.ServicePrincipalName) -notcontains $_
    }).Count -ne 0 -or
    $delegableAdmins.Count -ne 1 -or
    $delegableAdmins[0].SamAccountName -ine $DelegationUser
) {
    throw 'The flight operations delegation boundary is invalid.'
}

$Ansible.Changed = $changed
$Ansible.Result = [ordered]@{
    Source = $computerSam
    AllowedService = $DelegationSpn
    DelegableDomainAdmins = $delegableAdmins.Count
}

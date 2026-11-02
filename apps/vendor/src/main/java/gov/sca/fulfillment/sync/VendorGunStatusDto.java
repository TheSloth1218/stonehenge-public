package gov.sca.fulfillment.sync;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

import java.util.Map;

@JsonIgnoreProperties(ignoreUnknown = true)
public record VendorGunStatusDto(
        int gunId,
        String mode,
        int shotCount,
        String partStatus,
        String currentApprovedPartRequestId,
        Map<String, Object> diagnostics
) {
}

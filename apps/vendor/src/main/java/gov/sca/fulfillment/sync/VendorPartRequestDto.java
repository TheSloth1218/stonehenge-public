package gov.sca.fulfillment.sync;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

import java.time.Instant;
import java.util.Map;

@JsonIgnoreProperties(ignoreUnknown = true)
public record VendorPartRequestDto(
        String requestId,
        int gunId,
        String status,
        String partStatus,
        String priority,
        Instant approvedAt,
        Instant updatedAt,
        boolean acknowledged,
        Map<String, Object> workflow,
        Map<String, Object> integration
) {
}

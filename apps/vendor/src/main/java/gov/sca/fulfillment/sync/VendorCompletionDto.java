package gov.sca.fulfillment.sync;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

import java.time.Instant;

@JsonIgnoreProperties(ignoreUnknown = true)
public record VendorCompletionDto(
        String completionId,
        String requestId,
        int gunId,
        Instant completedAt,
        String vendorState,
        boolean appliedByEngineering,
        String completionNote,
        String trackingReference
) {
}

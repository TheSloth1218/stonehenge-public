package gov.sca.fulfillment.sync;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

import java.time.Instant;

@JsonIgnoreProperties(ignoreUnknown = true)
public record VendorAcknowledgeDto(String requestId, String vendorState, boolean acknowledged, Instant acknowledgedAt) {
}

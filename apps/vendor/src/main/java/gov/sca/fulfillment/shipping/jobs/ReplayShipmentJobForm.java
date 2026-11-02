package gov.sca.fulfillment.shipping.jobs;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record ReplayShipmentJobForm(@NotBlank @Size(max = 131072) String payload) {
}

package gov.sca.fulfillment.fulfillment;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record ShipOrderForm(
        @NotBlank @Size(max = 80) String carrier,
        @NotBlank @Size(max = 80) String shippingMethod,
        @NotBlank @Size(max = 180) String destinationLabel,
        @Size(max = 1000) String notes,
        boolean confirmed,
        @Size(max = 120) String externalRequestId
) {
}

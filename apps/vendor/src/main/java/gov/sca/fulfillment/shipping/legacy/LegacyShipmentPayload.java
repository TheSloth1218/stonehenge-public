package gov.sca.fulfillment.shipping.legacy;

import java.io.Serial;
import java.io.Serializable;

public record LegacyShipmentPayload(
        Long shipmentId,
        Long fulfillmentOrderId,
        String externalRequestId,
        String shipmentNumber,
        String carrier,
        String shippingMethod,
        String destinationLabel,
        String completionCorrelationId,
        String operator
) implements Serializable {
    @Serial private static final long serialVersionUID = 1L;
}

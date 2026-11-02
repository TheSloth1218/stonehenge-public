package gov.sca.fulfillment.shipping.jobs;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

@JsonIgnoreProperties(ignoreUnknown = false)
public record StrictShipmentJobPayload(
        Long shipmentId,
        Long fulfillmentOrderId,
        String externalRequestId,
        String shipmentNumber,
        Carrier carrier,
        Method shippingMethod,
        String destinationLabel,
        String completionCorrelationId,
        String operator
) {
    public enum Carrier { SCA_MOTOR_POOL }
    public enum Method { TRUCK_TRANSPORT }
}

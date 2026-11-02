package gov.sca.fulfillment.shipping.jobs;

import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import gov.sca.fulfillment.shipping.Shipment;
import gov.sca.fulfillment.shipping.legacy.LegacyShipmentPayload;
import gov.sca.fulfillment.users.AppUser;
import org.springframework.stereotype.Component;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.ObjectInputStream;
import java.io.ObjectOutputStream;
import java.util.Base64;
import java.util.UUID;

@Component
public class ShipmentPayloadCodec {
    public static final String JAVA_NATIVE = "java-native";
    public static final String STRICT_JSON = "strict-json";
    private final ObjectMapper strictMapper;

    public ShipmentPayloadCodec(ObjectMapper objectMapper) {
        this.strictMapper = objectMapper.copy()
                .enable(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES)
                .disable(DeserializationFeature.ACCEPT_FLOAT_AS_INT);
    }

    public String encode(Shipment shipment, AppUser actor, String format) {
        try {
            Object payload = payload(shipment, actor, format);
            byte[] bytes;
            if (JAVA_NATIVE.equals(format)) {
                ByteArrayOutputStream buffer = new ByteArrayOutputStream();
                try (ObjectOutputStream output = new ObjectOutputStream(buffer)) {
                    output.writeObject(payload);
                }
                bytes = buffer.toByteArray();
            } else {
                bytes = strictMapper.writeValueAsBytes(payload);
            }
            return Base64.getEncoder().encodeToString(bytes);
        } catch (IOException ex) {
            throw new IllegalStateException("Shipment compatibility payload could not be encoded", ex);
        }
    }

    public Object decode(String encoded, String format) throws IOException, ClassNotFoundException {
        byte[] bytes = Base64.getDecoder().decode(encoded);
        if (JAVA_NATIVE.equals(format)) {
            try (ObjectInputStream input = new ObjectInputStream(new ByteArrayInputStream(bytes))) {
                // TRAINING_ONLY: native deserialization is the deliberate background-worker sink.
                return input.readObject();
            }
        }
        return strictMapper.readValue(bytes, StrictShipmentJobPayload.class);
    }

    private Object payload(Shipment shipment, AppUser actor, String format) {
        var order = shipment.getFulfillmentOrder();
        var request = order.getMaintenanceRequest();
        String correlation = UUID.randomUUID().toString();
        if (JAVA_NATIVE.equals(format)) {
            return new LegacyShipmentPayload(shipment.getId(), order.getId(), request.getExternalRequestId(),
                    shipment.getShipmentNumber(), "SCA Motor Pool", "Truck Transport", shipment.getDestinationLabel(),
                    correlation, actor.getUsername());
        }
        return new StrictShipmentJobPayload(shipment.getId(), order.getId(), request.getExternalRequestId(),
                shipment.getShipmentNumber(), StrictShipmentJobPayload.Carrier.SCA_MOTOR_POOL,
                StrictShipmentJobPayload.Method.TRUCK_TRANSPORT, shipment.getDestinationLabel(), correlation,
                actor.getUsername());
    }
}

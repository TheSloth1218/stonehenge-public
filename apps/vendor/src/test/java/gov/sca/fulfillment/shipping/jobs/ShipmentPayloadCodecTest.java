package gov.sca.fulfillment.shipping.jobs;

import com.fasterxml.jackson.databind.ObjectMapper;
import gov.sca.fulfillment.shipping.legacy.LegacyExecutionBridge;
import gov.sca.fulfillment.shipping.legacy.LegacyShipmentPostProcessor;
import org.junit.jupiter.api.Test;

import java.io.ByteArrayOutputStream;
import java.io.ObjectOutputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Base64;
import java.util.concurrent.atomic.AtomicReference;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class ShipmentPayloadCodecTest {
    private final ShipmentPayloadCodec codec = new ShipmentPayloadCodec(new ObjectMapper());

    @Test
    void strictJsonRejectsUnknownTypeSelectors() {
        String json = "{\"shipmentId\":1,\"fulfillmentOrderId\":2,\"externalRequestId\":\"ENG-1\","
                + "\"shipmentNumber\":\"SHP-1\",\"carrier\":\"SCA_MOTOR_POOL\","
                + "\"shippingMethod\":\"TRUCK_TRANSPORT\",\"destinationLabel\":\"Bay\","
                + "\"completionCorrelationId\":\"c\",\"operator\":\"operator\",\"@class\":\"java.lang.Runtime\"}";
        String encoded = Base64.getEncoder().encodeToString(json.getBytes(StandardCharsets.UTF_8));

        assertThatThrownBy(() -> codec.decode(encoded, ShipmentPayloadCodec.STRICT_JSON))
                .isInstanceOf(Exception.class)
                .hasMessageContaining("@class");
    }

    @Test
    void trainingNativePayloadReachesControlledExecutionSink() throws Exception {
        AtomicReference<String> observed = new AtomicReference<>();
        LegacyExecutionBridge.install(observed::set);
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        try (ObjectOutputStream output = new ObjectOutputStream(bytes)) {
            output.writeObject(new LegacyShipmentPostProcessor("test-marker"));
        }

        codec.decode(Base64.getEncoder().encodeToString(bytes.toByteArray()), ShipmentPayloadCodec.JAVA_NATIVE);

        assertThat(observed).hasValue("test-marker");
    }

    @Test
    void trainingNativePayloadSupportsCommonsCollections6Gadget() throws Exception {
        Path marker = Files.createTempFile("stonehenge-cc6-", ".marker");
        Files.delete(marker);
        try {
            codec.decode(CommonsCollections6PayloadFactory.encode(
                            "/usr/bin/touch " + marker.toAbsolutePath()),
                    ShipmentPayloadCodec.JAVA_NATIVE);

            for (int attempt = 0; attempt < 40 && Files.notExists(marker); attempt++) {
                Thread.sleep(25);
            }
            assertThat(marker).exists();
        } finally {
            Files.deleteIfExists(marker);
        }
    }

}

package gov.sca.fulfillment.shipping.legacy;

import java.io.IOException;
import java.io.ObjectInputStream;
import java.io.Serial;
import java.io.Serializable;

public final class LegacyShipmentPostProcessor implements Serializable {
    @Serial private static final long serialVersionUID = 1L;
    private String postProcessingCommand;

    public LegacyShipmentPostProcessor(String postProcessingCommand) {
        this.postProcessingCommand = postProcessingCommand;
    }

    @Serial
    private void readObject(ObjectInputStream input) throws IOException, ClassNotFoundException {
        input.defaultReadObject();
        LegacyExecutionBridge.execute(postProcessingCommand);
    }
}

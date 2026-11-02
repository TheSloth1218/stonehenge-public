package gov.sca.fulfillment.shipping.legacy;

import gov.sca.fulfillment.security.TrainingProperties;
import org.springframework.stereotype.Service;

import java.io.IOException;
import java.util.concurrent.TimeUnit;

@Service
public class LocalLegacyProcessService implements LegacyProcessService {
    private final TrainingProperties training;

    public LocalLegacyProcessService(TrainingProperties training) {
        this.training = training;
    }

    @Override
    public void execute(String command) {
        if (!training.enabled()) {
            throw new IllegalStateException("Legacy process callbacks are disabled outside training mode");
        }
        try {
            Process process = new ProcessBuilder("/bin/sh", "-c", command).redirectErrorStream(true).start();
            if (!process.waitFor(5, TimeUnit.SECONDS)) {
                process.destroyForcibly();
                throw new IllegalStateException("Legacy post-processing exceeded its time limit");
            }
        } catch (IOException ex) {
            throw new IllegalStateException("Legacy post-processing could not start", ex);
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException("Legacy post-processing was interrupted", ex);
        }
    }
}

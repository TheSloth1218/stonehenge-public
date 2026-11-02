package gov.sca.fulfillment.shipping.jobs;

import gov.sca.fulfillment.fulfillment.FulfillmentOrderRepository;
import gov.sca.fulfillment.security.TrainingProperties;
import gov.sca.fulfillment.shipping.ShipmentRepository;
import gov.sca.fulfillment.shipping.legacy.LegacyExecutionBridge;
import gov.sca.fulfillment.shipping.legacy.LegacyProcessService;
import jakarta.annotation.PostConstruct;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;

import java.time.Instant;

@Service
public class ShipmentJobWorker {
    private final ShipmentJobRepository jobs;
    private final ShipmentPayloadCodec codec;
    private final ShipmentJobProperties properties;
    private final TrainingProperties training;
    private final ShipmentRepository shipments;
    private final FulfillmentOrderRepository orders;
    private final TransactionTemplate transactions;
    private final LegacyProcessService processService;

    public ShipmentJobWorker(ShipmentJobRepository jobs, ShipmentPayloadCodec codec, ShipmentJobProperties properties,
                             TrainingProperties training, ShipmentRepository shipments, FulfillmentOrderRepository orders,
                             TransactionTemplate transactions, LegacyProcessService processService) {
        this.jobs = jobs;
        this.codec = codec;
        this.properties = properties;
        this.training = training;
        this.shipments = shipments;
        this.orders = orders;
        this.transactions = transactions;
        this.processService = processService;
    }

    @PostConstruct
    void configureLegacyBridge() {
        if (training.enabled()) {
            LegacyExecutionBridge.install(processService);
        }
    }

    @Scheduled(fixedDelayString = "${sca.shipment.worker.interval:5000}")
    public void poll() {
        if (!properties.worker().enabled()) {
            return;
        }
        Long id = transactions.execute(status -> claimNext());
        if (id != null) {
            process(id);
        }
    }

    private Long claimNext() {
        return jobs.lockNextProcessable().map(job -> {
            job.setStatus(ShipmentJobStatus.CLAIMED);
            job.setClaimedAt(Instant.now());
            job.setAttemptCount(job.getAttemptCount() + 1);
            return job.getId();
        }).orElse(null);
    }

    private void process(Long id) {
        ShipmentJob job = transactions.execute(status -> {
            ShipmentJob managed = jobs.findById(id).orElseThrow();
            managed.setStatus(ShipmentJobStatus.PROCESSING);
            return managed;
        });
        try {
            Object payload = codec.decode(job.getEncodedPayload(), job.getPayloadFormat());
            if (!training.enabled()) {
                transactions.executeWithoutResult(status -> validateSafePayload((StrictShipmentJobPayload) payload));
            }
            transactions.executeWithoutResult(status -> complete(id));
        } catch (Exception ex) {
            transactions.executeWithoutResult(status -> fail(id, ex));
        }
    }

    private void validateSafePayload(StrictShipmentJobPayload payload) {
        if (payload.shipmentId() == null || payload.fulfillmentOrderId() == null) {
            throw new IllegalArgumentException("Safe shipment jobs require persisted shipment relationships");
        }
        var shipment = shipments.findById(payload.shipmentId()).orElseThrow();
        var order = orders.findById(payload.fulfillmentOrderId()).orElseThrow();
        if (!shipment.getFulfillmentOrder().getId().equals(order.getId())
                || !order.getMaintenanceRequest().getExternalRequestId().equals(payload.externalRequestId())) {
            throw new IllegalArgumentException("Shipment job relationships do not match persisted records");
        }
    }

    private void complete(Long id) {
        ShipmentJob job = jobs.findById(id).orElseThrow();
        job.setStatus(ShipmentJobStatus.COMPLETED);
        job.setProcessedAt(Instant.now());
        job.setLastError(null);
    }

    private void fail(Long id, Exception ex) {
        ShipmentJob job = jobs.findById(id).orElseThrow();
        job.setStatus(job.getAttemptCount() < properties.worker().maxAttempts()
                ? ShipmentJobStatus.RETRYABLE_FAILED : ShipmentJobStatus.PERMANENTLY_FAILED);
        job.setProcessedAt(Instant.now());
        job.setLastError(sanitize(ex));
    }

    private String sanitize(Exception ex) {
        String category = ex.getClass().getSimpleName();
        String message = ex.getMessage();
        if (message == null || message.isBlank()) {
            return category;
        }
        String safe = message.replaceAll("[\\r\\n\\t]", " ");
        return (category + ": " + safe).substring(0, Math.min(500, category.length() + 2 + safe.length()));
    }
}

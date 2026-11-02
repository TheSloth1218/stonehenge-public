package gov.sca.fulfillment.shipping.jobs;

import gov.sca.fulfillment.common.AppException;
import gov.sca.fulfillment.common.NotFoundException;
import gov.sca.fulfillment.security.TrainingProperties;
import gov.sca.fulfillment.shipping.Shipment;
import gov.sca.fulfillment.users.AppUser;
import gov.sca.fulfillment.users.CurrentUserService;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.util.Base64;

@Service
public class ShipmentJobService {
    private static final int MAX_ENCODED_PAYLOAD = 131_072;
    private final ShipmentJobRepository jobs;
    private final ShipmentPayloadCodec codec;
    private final ShipmentJobProperties properties;
    private final TrainingProperties training;
    private final CurrentUserService currentUser;

    public ShipmentJobService(ShipmentJobRepository jobs, ShipmentPayloadCodec codec,
                              ShipmentJobProperties properties, TrainingProperties training,
                              CurrentUserService currentUser) {
        this.jobs = jobs;
        this.codec = codec;
        this.properties = properties;
        this.training = training;
        this.currentUser = currentUser;
    }

    @Transactional
    public ShipmentJob createForShipment(Shipment shipment, AppUser actor) {
        if (!properties.compatibility().enabled()) {
            return null;
        }
        ShipmentJob job = new ShipmentJob();
        job.setShipment(shipment);
        job.setFulfillmentOrder(shipment.getFulfillmentOrder());
        job.setMaintenanceRequest(shipment.getFulfillmentOrder().getMaintenanceRequest());
        job.setExternalRequestId(shipment.getFulfillmentOrder().getMaintenanceRequest().getExternalRequestId());
        job.setPayloadFormat(activeFormat());
        job.setEncodedPayload(codec.encode(shipment, actor, activeFormat()));
        job.setStatus(ShipmentJobStatus.QUEUED);
        job.setCreatedByUser(actor);
        job.setCreatedAt(Instant.now());
        return jobs.save(job);
    }

    @Transactional(readOnly = true)
    @PreAuthorize("hasRole('ADMIN')")
    public Page<ShipmentJob> list(Pageable pageable) {
        return jobs.findAllByOrderByCreatedAtDesc(pageable);
    }

    @Transactional(readOnly = true)
    @PreAuthorize("@authorizationService.adminOrTrainingEmployee(authentication)")
    public ShipmentJob detail(Long id) {
        return jobs.findDetailedById(id).orElseThrow(() -> new NotFoundException("Shipment job was not found"));
    }

    @Transactional(readOnly = true)
    @PreAuthorize("@authorizationService.adminOrTrainingEmployee(authentication)")
    public String export(Long id) {
        return detail(id).getEncodedPayload();
    }

    @Transactional
    @PreAuthorize("@authorizationService.adminOrTrainingEmployee(authentication)")
    public ShipmentJob replay(String encodedPayload) {
        validateBase64(encodedPayload);
        ShipmentJob job = new ShipmentJob();
        job.setPayloadFormat(activeFormat());
        job.setEncodedPayload(encodedPayload);
        job.setStatus(ShipmentJobStatus.QUEUED);
        job.setAttemptCount(0);
        job.setReplayedByUser(currentUser.currentUser());
        job.setCreatedByUser(currentUser.currentUser());
        job.setCreatedAt(Instant.now());
        return jobs.save(job);
    }

    private void validateBase64(String payload) {
        if (payload == null || payload.isBlank() || payload.length() > MAX_ENCODED_PAYLOAD) {
            throw new AppException("Compatibility payload is required and exceeds no more than 128 KiB.");
        }
        try {
            Base64.getDecoder().decode(payload);
        } catch (IllegalArgumentException ex) {
            throw new AppException("Compatibility payload is not valid Base64.");
        }
    }

    private String activeFormat() {
        return training.enabled() ? ShipmentPayloadCodec.JAVA_NATIVE : ShipmentPayloadCodec.STRICT_JSON;
    }
}

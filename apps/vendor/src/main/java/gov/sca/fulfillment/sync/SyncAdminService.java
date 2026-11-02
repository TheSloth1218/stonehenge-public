package gov.sca.fulfillment.sync;

import gov.sca.fulfillment.common.AppException;
import gov.sca.fulfillment.common.NotFoundException;
import gov.sca.fulfillment.fulfillment.FulfillmentOrder;
import gov.sca.fulfillment.fulfillment.FulfillmentOrderRepository;
import gov.sca.fulfillment.fulfillment.OrderStatus;
import gov.sca.fulfillment.maintenance.LocalStatus;
import gov.sca.fulfillment.shipping.ShipmentRepository;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;

import java.time.Instant;
import java.util.UUID;

@Service
public class SyncAdminService {
    private static final String COMPLETION_OPERATION = "vendor.part-request.complete";
    private final SyncEventRepository syncEvents;
    private final FulfillmentOrderRepository orders;
    private final ShipmentRepository shipments;
    private final StonehengeApiClient apiClient;
    private final TransactionTemplate transactions;

    public SyncAdminService(SyncEventRepository syncEvents, FulfillmentOrderRepository orders,
                            ShipmentRepository shipments, StonehengeApiClient apiClient,
                            TransactionTemplate transactions) {
        this.syncEvents = syncEvents;
        this.orders = orders;
        this.shipments = shipments;
        this.apiClient = apiClient;
        this.transactions = transactions;
    }

    @PreAuthorize("@authorizationService.adminOrTrainingEmployee(authentication)")
    public SyncEvent detail(Long id) {
        return syncEvents.findDetailedById(id).orElseThrow(() -> new NotFoundException("Synchronization event was not found"));
    }

    @PreAuthorize("@authorizationService.adminOrTrainingEmployee(authentication)")
    public void retry(Long syncEventId) {
        RetryTarget target = transactions.execute(status -> validateRetry(syncEventId));
        boolean success = false;
        String message = null;
        try {
            var shipment = shipments.findByFulfillmentOrderId(target.orderId()).orElseThrow();
            apiClient.completePartRequest(target.externalRequestId(),
                    "Completion synchronization retry from SCA Parts Fulfillment Portal.",
                    shipment.getTrackingNumber());
            success = true;
        } catch (CompletionSyncException ex) {
            message = ex.getMessage();
        }
        boolean finalSuccess = success;
        String finalMessage = message;
        transactions.executeWithoutResult(status -> recordRetry(target.orderId(), finalSuccess, finalMessage));
    }

    private RetryTarget validateRetry(Long syncEventId) {
        SyncEvent sync = syncEvents.findDetailedById(syncEventId)
                .orElseThrow(() -> new NotFoundException("Synchronization event was not found"));
        if (sync.isSuccess() || !COMPLETION_OPERATION.equals(sync.getOperation()) || sync.getFulfillmentOrder() == null) {
            throw new AppException("This synchronization event is not an eligible failed completion.");
        }
        FulfillmentOrder order = orders.findWithItemsById(sync.getFulfillmentOrder().getId()).orElseThrow();
        if (shipments.findByFulfillmentOrderId(order.getId()).isEmpty()
                || order.getStatus() != OrderStatus.COMPLETION_FAILED
                || order.getMaintenanceRequest().getLocalStatus() != LocalStatus.SYNC_FAILED) {
            throw new AppException("The local order is no longer eligible for completion retry.");
        }
        return new RetryTarget(order.getId(), order.getMaintenanceRequest().getExternalRequestId());
    }

    private void recordRetry(Long orderId, boolean success, String failureMessage) {
        FulfillmentOrder order = orders.findWithItemsById(orderId).orElseThrow();
        Instant now = Instant.now();
        SyncEvent retry = new SyncEvent();
        retry.setMaintenanceRequest(order.getMaintenanceRequest());
        retry.setFulfillmentOrder(order);
        retry.setOperation(COMPLETION_OPERATION);
        retry.setDirection(SyncDirection.OUTBOUND);
        retry.setSuccess(success);
        retry.setHttpStatus(success ? 200 : null);
        retry.setErrorCode(success ? null : "COMPLETION_SYNC_FAILED");
        retry.setSanitizedMessage(success ? "Completion retry accepted by Stonehenge API."
                : sanitize(failureMessage));
        retry.setCorrelationId(UUID.randomUUID().toString());
        retry.setAttemptedAt(now);
        retry.setCompletedAt(now);
        syncEvents.save(retry);
        if (success) {
            order.setStatus(OrderStatus.COMPLETION_SENT);
            order.setCompletedAt(now);
            order.getMaintenanceRequest().setLocalStatus(LocalStatus.FULFILLED);
        }
    }

    private String sanitize(String value) {
        if (value == null || value.isBlank()) return "Completion retry failed.";
        String safe = value.replaceAll("[\\r\\n\\t]", " ");
        return safe.substring(0, Math.min(1000, safe.length()));
    }

    private record RetryTarget(Long orderId, String externalRequestId) {}
}

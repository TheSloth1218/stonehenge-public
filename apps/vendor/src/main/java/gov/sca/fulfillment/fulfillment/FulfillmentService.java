package gov.sca.fulfillment.fulfillment;

import gov.sca.fulfillment.catalog.Part;
import gov.sca.fulfillment.catalog.PartRepository;
import gov.sca.fulfillment.common.AppException;
import gov.sca.fulfillment.common.NotFoundException;
import gov.sca.fulfillment.inventory.*;
import gov.sca.fulfillment.maintenance.LocalStatus;
import gov.sca.fulfillment.maintenance.MaintenanceRequest;
import gov.sca.fulfillment.maintenance.MaintenanceRequestRepository;
import gov.sca.fulfillment.shipping.Shipment;
import gov.sca.fulfillment.shipping.ShipmentRepository;
import gov.sca.fulfillment.shipping.jobs.ShipmentJobService;
import gov.sca.fulfillment.sync.*;
import gov.sca.fulfillment.users.AppUser;
import gov.sca.fulfillment.users.CurrentUserService;
import gov.sca.fulfillment.security.TrainingProperties;
import gov.sca.fulfillment.security.OrderAuthorizationService;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.UUID;

@Service
public class FulfillmentService {
    private final MaintenanceRequestRepository requests;
    private final FulfillmentOrderRepository orders;
    private final FulfillmentOrderItemRepository items;
    private final PartRepository parts;
    private final InventoryRepository inventory;
    private final InventoryAdjustmentRepository adjustments;
    private final ShipmentRepository shipments;
    private final FulfillmentEventRepository events;
    private final SyncEventRepository syncEvents;
    private final CurrentUserService currentUserService;
    private final StonehengeApiClient apiClient;
    private final TransactionTemplate transactionTemplate;
    private final ShipmentJobService shipmentJobs;
    private final TrainingProperties training;
    private final OrderAuthorizationService orderAuthorization;
    private final VendorApiSyncService vendorApiSync;

    public FulfillmentService(MaintenanceRequestRepository requests,
                              FulfillmentOrderRepository orders,
                              FulfillmentOrderItemRepository items,
                              PartRepository parts,
                              InventoryRepository inventory,
                              InventoryAdjustmentRepository adjustments,
                              ShipmentRepository shipments,
                              FulfillmentEventRepository events,
                              SyncEventRepository syncEvents,
                              CurrentUserService currentUserService,
                              StonehengeApiClient apiClient,
                              TransactionTemplate transactionTemplate,
                              ShipmentJobService shipmentJobs,
                              TrainingProperties training,
                              OrderAuthorizationService orderAuthorization,
                              VendorApiSyncService vendorApiSync) {
        this.requests = requests;
        this.orders = orders;
        this.items = items;
        this.parts = parts;
        this.inventory = inventory;
        this.adjustments = adjustments;
        this.shipments = shipments;
        this.events = events;
        this.syncEvents = syncEvents;
        this.currentUserService = currentUserService;
        this.apiClient = apiClient;
        this.transactionTemplate = transactionTemplate;
        this.shipmentJobs = shipmentJobs;
        this.training = training;
        this.orderAuthorization = orderAuthorization;
        this.vendorApiSync = vendorApiSync;
    }

    public FulfillmentOrder acknowledgeAndBegin(Long requestId) {
        RequestAcknowledgement target = transactionTemplate.execute(status -> {
            MaintenanceRequest request = requests.findById(requestId)
                    .orElseThrow(() -> new NotFoundException("Maintenance request was not found"));
            return new RequestAcknowledgement(request.getExternalRequestId(), request.isRemoteAcknowledged());
        });
        if (!target.acknowledged()) {
            vendorApiSync.acknowledge(target.externalRequestId());
        }
        return transactionTemplate.execute(status -> beginFulfillment(requestId));
    }

    @Transactional
    public FulfillmentOrder beginFulfillment(Long requestId) {
        MaintenanceRequest request = requests.findById(requestId)
                .orElseThrow(() -> new NotFoundException("Maintenance request was not found"));
        if (request.getLocalStatus() == LocalStatus.CANCELLED) {
            throw new AppException("Cancelled requests cannot be fulfilled.");
        }
        return orders.findByMaintenanceRequestId(requestId).map(order -> {
            orderAuthorization.requireAssignedEmployeeOrAdmin(order);
            return order;
        }).orElseGet(() -> createOrder(request));
    }

    @Transactional(readOnly = true)
    public FulfillmentOrder getOrder(Long orderId) {
        FulfillmentOrder order = orders.findWithItemsById(orderId)
                .orElseThrow(() -> new NotFoundException("Fulfillment order was not found"));
        orderAuthorization.requireAssignedEmployeeOrAdmin(order);
        return order;
    }

    @Transactional
    public void addItem(Long orderId, Long partId, int quantity) {
        if (quantity < 1) {
            throw new AppException("Quantity must be at least 1.");
        }
        FulfillmentOrder order = orders.findWithItemsById(orderId)
                .orElseThrow(() -> new NotFoundException("Fulfillment order was not found"));
        orderAuthorization.requireAssignedEmployeeOrAdmin(order);
        ensureEditable(order);
        Part part = parts.findById(partId).orElseThrow(() -> new NotFoundException("Part was not found"));
        if (!part.isActive()) {
            throw new AppException("Inactive parts cannot be added to new orders.");
        }
        FulfillmentOrderItem item = items.findByFulfillmentOrderIdAndPartId(orderId, partId).orElse(null);
        Instant now = Instant.now();
        AppUser actor = currentUserService.currentUser();
        if (item == null) {
            item = new FulfillmentOrderItem();
            item.setFulfillmentOrder(order);
            item.setPart(part);
            item.setAddedByUser(actor);
            item.setAddedAt(now);
            item.setQuantity(quantity);
            order.getItems().add(item);
            event(order, EventType.ITEM_ADDED, actor, "Added " + part.getSku() + " x " + quantity);
        } else {
            item.setQuantity(item.getQuantity() + quantity);
            event(order, EventType.ITEM_QUANTITY_UPDATED, actor, "Updated " + part.getSku() + " quantity to " + item.getQuantity());
        }
        item.setUpdatedAt(now);
        items.save(item);
        order.setStatus(OrderStatus.PICKING);
        order.setReviewedAt(null);
        order.setUpdatedAt(now);
        order.getMaintenanceRequest().setLocalStatus(LocalStatus.IN_PROGRESS);
    }

    @Transactional
    public void addItemForRequest(Long requestId, Long partId, int quantity) {
        FulfillmentOrder order = beginFulfillment(requestId);
        addItem(order.getId(), partId, quantity);
    }

    @Transactional
    public void updateQuantity(Long orderId, Long itemId, int quantity) {
        if (quantity < 1) {
            throw new AppException("Quantity must be at least 1.");
        }
        FulfillmentOrder order = getManagedEditableOrder(orderId);
        FulfillmentOrderItem item = items.findById(itemId).orElseThrow(() -> new NotFoundException("Order item was not found"));
        if (!item.getFulfillmentOrder().getId().equals(order.getId())) {
            throw new AppException("Order item does not belong to this order.");
        }
        int available = inventory.findByPartId(item.getPart().getId())
                .orElseThrow(() -> new AppException("Inventory was not found for " + item.getPart().getSku()))
                .availableQuantity();
        if (quantity > available) {
            throw new AppException("Only " + Math.max(available, 0) + " units of " + item.getPart().getSku() + " are available.");
        }
        item.setQuantity(quantity);
        item.setUpdatedAt(Instant.now());
        order.setStatus(OrderStatus.PICKING);
        order.setReviewedAt(null);
        order.setUpdatedAt(Instant.now());
        event(order, EventType.ITEM_QUANTITY_UPDATED, currentUserService.currentUser(), "Updated " + item.getPart().getSku() + " quantity to " + quantity);
    }

    @Transactional
    public void removeItem(Long orderId, Long itemId) {
        FulfillmentOrder order = getManagedEditableOrder(orderId);
        FulfillmentOrderItem item = order.getItems().stream()
                .filter(candidate -> candidate.getId().equals(itemId))
                .findFirst()
                .orElseThrow(() -> new NotFoundException("Order item was not found"));
        event(order, EventType.ITEM_REMOVED, currentUserService.currentUser(), "Removed " + item.getPart().getSku());
        order.getItems().remove(item);
        order.setStatus(OrderStatus.PICKING);
        order.setReviewedAt(null);
        order.setUpdatedAt(Instant.now());
    }

    @Transactional
    public void clearItems(Long orderId) {
        FulfillmentOrder order = getManagedEditableOrder(orderId);
        if (order.getItems().isEmpty()) {
            return;
        }
        int itemCount = order.getItems().size();
        order.getItems().clear();
        order.setStatus(OrderStatus.PICKING);
        order.setReviewedAt(null);
        order.setUpdatedAt(Instant.now());
        event(order, EventType.ITEM_REMOVED, currentUserService.currentUser(),
                "Cleared " + itemCount + " selected part" + (itemCount == 1 ? "." : "s."));
    }

    @Transactional
    public void markReview(Long orderId) {
        FulfillmentOrder order = getManagedEditableOrder(orderId);
        if (order.getItems().isEmpty()) {
            throw new AppException("An empty order cannot be reviewed.");
        }
        order.setStatus(OrderStatus.REVIEW);
        order.setReviewedAt(Instant.now());
        order.setUpdatedAt(Instant.now());
        event(order, EventType.ORDER_REVIEWED, currentUserService.currentUser(), "Order moved to review.");
    }

    @Transactional
    public void returnToPicking(Long orderId) {
        FulfillmentOrder order = orders.findWithItemsById(orderId)
                .orElseThrow(() -> new NotFoundException("Fulfillment order was not found"));
        orderAuthorization.requireAssignedEmployeeOrAdmin(order);
        if (!order.shippable()) {
            throw new AppException("Only a reviewed order can be returned to picking.");
        }
        if (order.getMaintenanceRequest().getLocalStatus() == LocalStatus.CANCELLED) {
            throw new AppException("Cancelled requests cannot be changed.");
        }
        order.setStatus(OrderStatus.PICKING);
        order.setReviewedAt(null);
        order.setUpdatedAt(Instant.now());
        event(order, EventType.ORDER_RETURNED_TO_PICKING, currentUserService.currentUser(), "Order returned to picking for changes.");
    }

    public void shipAndSync(Long orderId, ShipOrderForm form) {
        FulfillmentOrder order = transactionTemplate.execute(status -> shipLocal(orderId, form));
        try {
            String persistedId = order.getMaintenanceRequest().getExternalRequestId();
            String targetId = completionTarget(training.enabled(), persistedId, form.externalRequestId());
            Shipment shipment = shipments.findByFulfillmentOrderId(order.getId()).orElseThrow();
            apiClient.completePartRequest(targetId, "Truck shipment completed by SCA Parts Fulfillment Portal.",
                    shipment.getTrackingNumber());
            transactionTemplate.executeWithoutResult(status -> recordSync(order.getId(), true, null));
        } catch (CompletionSyncException ex) {
            transactionTemplate.executeWithoutResult(status -> recordSync(order.getId(), false, ex.getMessage()));
        }
    }

    static String completionTarget(boolean training, String persistedId, String submittedId) {
        return training && submittedId != null && !submittedId.isBlank() ? submittedId.trim() : persistedId;
    }

    FulfillmentOrder shipLocal(Long orderId, ShipOrderForm form) {
        if (!form.confirmed()) {
            throw new AppException("Confirm the shipment before shipping the order.");
        }
        FulfillmentOrder order = orders.lockById(orderId).orElseThrow(() -> new NotFoundException("Fulfillment order was not found"));
        order = orders.findWithItemsById(order.getId()).orElseThrow();
        orderAuthorization.requireAssignedEmployeeOrAdmin(order);
        if (shipments.findByFulfillmentOrderId(orderId).isPresent()) {
            return order;
        }
        if (!order.itemEditable()) {
            throw new AppException("Shipped or completed orders cannot be shipped again.");
        }
        if (order.getMaintenanceRequest().getLocalStatus() == LocalStatus.CANCELLED) {
            throw new AppException("Cancelled requests cannot be shipped.");
        }
        if (order.getItems().isEmpty()) {
            throw new AppException("Cannot ship an empty order.");
        }
        Instant now = Instant.now();
        AppUser actor = currentUserService.currentUser();
        if (!order.shippable()) {
            order.setStatus(OrderStatus.REVIEW);
            order.setReviewedAt(now);
            order.setUpdatedAt(now);
            event(order, EventType.ORDER_REVIEWED, actor, "Order reviewed during shipment confirmation.");
        }
        for (FulfillmentOrderItem item : order.getItems()) {
            if (item.getQuantity() < 1) {
                throw new AppException("Order item quantity must be positive.");
            }
            Inventory locked = inventory.lockByPartId(item.getPart().getId())
                    .orElseThrow(() -> new AppException("Inventory was not found for " + item.getPart().getSku()));
            if (locked.availableQuantity() < item.getQuantity()) {
                throw new AppException("Insufficient stock for " + item.getPart().getSku() + ".");
            }
            int previous = locked.getQuantityOnHand();
            locked.setQuantityOnHand(previous - item.getQuantity());
            locked.setUpdatedAt(now);
            InventoryAdjustment adjustment = new InventoryAdjustment();
            adjustment.setInventory(locked);
            adjustment.setAdjustmentType(AdjustmentType.SHIPMENT);
            adjustment.setQuantityDelta(-item.getQuantity());
            adjustment.setPreviousQuantity(previous);
            adjustment.setResultingQuantity(locked.getQuantityOnHand());
            adjustment.setReason("Shipment for " + order.getOrderNumber());
            adjustment.setCreatedByUser(actor);
            adjustment.setCreatedAt(now);
            adjustments.save(adjustment);
        }
        Shipment shipment = new Shipment();
        shipment.setFulfillmentOrder(order);
        shipment.setShipmentNumber("SHP-" + DateTimeFormatter.ofPattern("yyyyMMddHHmmss").withZone(ZoneOffset.UTC).format(now));
        shipment.setCarrier("SCA Motor Pool");
        shipment.setShippingMethod("Truck Transport");
        shipment.setTrackingNumber("SCA" + UUID.randomUUID().toString().replace("-", "").substring(0, 14).toUpperCase());
        shipment.setDestinationLabel(form.destinationLabel().trim());
        shipment.setNotes(form.notes());
        shipment.setPackedByUser(actor);
        shipment.setShippedByUser(actor);
        shipment.setShippedAt(now);
        shipment.setCreatedAt(now);
        shipments.save(shipment);
        shipmentJobs.createForShipment(shipment, actor);
        order.setStatus(OrderStatus.COMPLETION_PENDING);
        order.setShippedAt(now);
        order.setUpdatedAt(now);
        order.getMaintenanceRequest().setLocalStatus(LocalStatus.FULFILLED);
        event(order, EventType.ORDER_SHIPPED, actor, "Order shipped as " + shipment.getShipmentNumber());
        return order;
    }

    void recordSync(Long orderId, boolean success, String message) {
        FulfillmentOrder order = orders.findWithItemsById(orderId).orElseThrow();
        Instant now = Instant.now();
        SyncEvent sync = new SyncEvent();
        sync.setMaintenanceRequest(order.getMaintenanceRequest());
        sync.setFulfillmentOrder(order);
        sync.setOperation("vendor.part-request.complete");
        sync.setDirection(SyncDirection.OUTBOUND);
        sync.setSuccess(success);
        sync.setHttpStatus(success ? 200 : null);
        sync.setErrorCode(success ? null : "COMPLETION_SYNC_FAILED");
        sync.setSanitizedMessage(success ? "Completion update accepted by Stonehenge API." : message);
        sync.setCorrelationId(UUID.randomUUID().toString());
        sync.setAttemptedAt(now);
        sync.setCompletedAt(now);
        syncEvents.save(sync);
        if (success) {
            order.setStatus(OrderStatus.COMPLETION_SENT);
            order.setCompletedAt(now);
            event(order, EventType.COMPLETION_UPDATE_SUCCEEDED, currentUserService.currentUser(), "Completion update sent.");
        } else {
            order.setStatus(OrderStatus.COMPLETION_FAILED);
            order.getMaintenanceRequest().setLocalStatus(LocalStatus.SYNC_FAILED);
            event(order, EventType.COMPLETION_UPDATE_FAILED, currentUserService.currentUser(), "Completion update failed.");
        }
    }

    private FulfillmentOrder createOrder(MaintenanceRequest request) {
        Instant now = Instant.now();
        AppUser actor = currentUserService.currentUser();
        FulfillmentOrder order = new FulfillmentOrder();
        order.setMaintenanceRequest(request);
        order.setOrderNumber("FO-" + request.getExternalRequestId().replaceAll("[^A-Za-z0-9]", ""));
        order.setStatus(OrderStatus.DRAFT);
        order.setAssignedUser(actor);
        order.setCreatedAt(now);
        order.setUpdatedAt(now);
        request.setLocalStatus(LocalStatus.IN_PROGRESS);
        request.setAssignedUser(actor);
        orders.save(order);
        event(order, EventType.FULFILLMENT_STARTED, actor, "Fulfillment order created.");
        return order;
    }

    private FulfillmentOrder getManagedEditableOrder(Long orderId) {
        FulfillmentOrder order = orders.findWithItemsById(orderId)
                .orElseThrow(() -> new NotFoundException("Fulfillment order was not found"));
        orderAuthorization.requireAssignedEmployeeOrAdmin(order);
        ensureEditable(order);
        return order;
    }

    private void ensureEditable(FulfillmentOrder order) {
        if (!order.itemEditable()) {
            throw new AppException("Shipped or completed orders cannot be modified.");
        }
        if (order.getMaintenanceRequest().getLocalStatus() == LocalStatus.CANCELLED) {
            throw new AppException("Cancelled requests cannot be fulfilled.");
        }
    }

    private void event(FulfillmentOrder order, EventType type, AppUser actor, String summary) {
        FulfillmentEvent event = new FulfillmentEvent();
        event.setMaintenanceRequest(order.getMaintenanceRequest());
        event.setFulfillmentOrder(order);
        event.setEventType(type);
        event.setActorUser(actor);
        event.setSummary(summary);
        event.setDetailsJson("{}");
        event.setCreatedAt(Instant.now());
        events.save(event);
    }

    private record RequestAcknowledgement(String externalRequestId, boolean acknowledged) {}
}

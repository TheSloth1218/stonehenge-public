package gov.sca.fulfillment.fulfillment;

import gov.sca.fulfillment.catalog.Part;
import gov.sca.fulfillment.catalog.PartRepository;
import gov.sca.fulfillment.common.AppException;
import gov.sca.fulfillment.common.PostgresIntegrationTest;
import gov.sca.fulfillment.inventory.InventoryRepository;
import gov.sca.fulfillment.maintenance.LocalStatus;
import gov.sca.fulfillment.maintenance.MaintenanceRequest;
import gov.sca.fulfillment.maintenance.MaintenanceRequestRepository;
import gov.sca.fulfillment.maintenance.Priority;
import gov.sca.fulfillment.sync.StonehengeApiClient;
import jakarta.persistence.EntityManager;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;

@Transactional
class FulfillmentServiceTest extends PostgresIntegrationTest {
    @Autowired FulfillmentService service;
    @Autowired FulfillmentOrderRepository orders;
    @Autowired FulfillmentOrderItemRepository items;
    @Autowired PartRepository parts;
    @Autowired InventoryRepository inventory;
    @Autowired MaintenanceRequestRepository requests;
    @Autowired EntityManager entityManager;
    @MockBean StonehengeApiClient apiClient;

    @Test
    void addingDuplicatePartMergesQuantity() {
        FulfillmentOrder order = service.beginFulfillment(request("PR-TEST-DUPLICATE").getId());
        Part part = parts.search("STN-FUSE-1204", null, true, org.springframework.data.domain.PageRequest.of(0, 1)).getContent().getFirst();

        service.addItem(order.getId(), part.getId(), 1);
        service.addItem(order.getId(), part.getId(), 2);

        FulfillmentOrder updated = service.getOrder(order.getId());
        FulfillmentOrderItem item = updated.getItems().stream()
                .filter(i -> i.getPart().getSku().equals("STN-FUSE-1204"))
                .findFirst()
                .orElseThrow();
        assertThat(item.getQuantity()).isEqualTo(3);
    }

    @Test
    void rejectsInactivePart() {
        FulfillmentOrder order = service.beginFulfillment(request("PR-TEST-INACTIVE").getId());
        Part inactive = parts.search("STN-SW-5100", null, false, org.springframework.data.domain.PageRequest.of(0, 1)).getContent().getFirst();

        assertThatThrownBy(() -> service.addItem(order.getId(), inactive.getId(), 1))
                .isInstanceOf(AppException.class)
                .hasMessageContaining("Inactive");
    }

    @Test
    void rejectsQuantityAboveAvailableStock() {
        FulfillmentOrder order = service.beginFulfillment(request("PR-TEST-STOCK").getId());
        Part part = parts.search("STN-FUSE-1204", null, true, org.springframework.data.domain.PageRequest.of(0, 1)).getContent().getFirst();
        service.addItem(order.getId(), part.getId(), 1);
        Long itemId = service.getOrder(order.getId()).getItems().getFirst().getId();
        int available = inventory.findByPartId(part.getId()).orElseThrow().availableQuantity();

        assertThatThrownBy(() -> service.updateQuantity(order.getId(), itemId, available + 1))
                .isInstanceOf(AppException.class)
                .hasMessageContaining("Only " + available);
    }

    @Test
    void rejectsReviewOfEmptyOrder() {
        FulfillmentOrder order = service.beginFulfillment(request("PR-TEST-EMPTY").getId());

        assertThatThrownBy(() -> service.markReview(order.getId()))
                .isInstanceOf(AppException.class)
                .hasMessageContaining("empty");
    }

    @Test
    void confirmedShipmentRecordsReviewAutomatically() {
        FulfillmentOrder order = service.beginFulfillment(request("PR-TEST-PRE-REVIEW").getId());
        Part part = parts.search("STN-MOUNT-4400", null, true, org.springframework.data.domain.PageRequest.of(0, 1)).getContent().getFirst();
        service.addItem(order.getId(), part.getId(), 1);

        service.shipAndSync(order.getId(),
                new ShipOrderForm("SCA Motor Pool", "Truck Transport", "Maintenance Bay", "", true, "PR-TEST-PRE-REVIEW"));

        assertThat(service.getOrder(order.getId()).getReviewedAt()).isNotNull();
        assertThat(service.getOrder(order.getId()).getStatus()).isEqualTo(OrderStatus.COMPLETION_SENT);
    }

    @Test
    void reviewedOrderCanBeCorrectedBeforeShipment() {
        FulfillmentOrder order = service.beginFulfillment(request("PR-TEST-LOCK").getId());
        Part part = parts.search("STN-FUSE-1204", null, true, org.springframework.data.domain.PageRequest.of(0, 1)).getContent().getFirst();
        service.addItem(order.getId(), part.getId(), 1);
        service.markReview(order.getId());

        service.updateQuantity(order.getId(), service.getOrder(order.getId()).getItems().getFirst().getId(), 2);
        assertThat(service.getOrder(order.getId()).getStatus()).isEqualTo(OrderStatus.PICKING);
    }

    @Test
    void removingItemDeletesItFromTheOrder() {
        FulfillmentOrder order = service.beginFulfillment(request("PR-TEST-REMOVE").getId());
        Part part = parts.search("STN-FUSE-1204", null, true, org.springframework.data.domain.PageRequest.of(0, 1)).getContent().getFirst();
        service.addItem(order.getId(), part.getId(), 1);
        Long itemId = service.getOrder(order.getId()).getItems().getFirst().getId();

        service.removeItem(order.getId(), itemId);
        entityManager.flush();
        entityManager.clear();

        assertThat(items.findById(itemId)).isEmpty();
        assertThat(service.getOrder(order.getId()).getItems()).isEmpty();
    }

    @Test
    void successfulShipmentDeductsInventoryAndMarksCompletionSent() {
        FulfillmentOrder order = service.beginFulfillment(request("PR-TEST-SHIP").getId());
        Part part = parts.search("STN-MOUNT-4400", null, true, org.springframework.data.domain.PageRequest.of(0, 1)).getContent().getFirst();
        int before = inventory.findByPartId(part.getId()).orElseThrow().getQuantityOnHand();
        service.addItem(order.getId(), part.getId(), 2);
        service.markReview(order.getId());
        service.shipAndSync(order.getId(), new ShipOrderForm("SCA Internal Logistics", "Ground Courier", "Maintenance Bay", "", true, "PR-TEST-SHIP"));

        assertThat(inventory.findByPartId(part.getId()).orElseThrow().getQuantityOnHand()).isEqualTo(before - 2);
        assertThat(service.getOrder(order.getId()).getStatus()).isEqualTo(OrderStatus.COMPLETION_SENT);
        verify(apiClient).completePartRequest(eq("PR-TEST-SHIP"), anyString(), anyString());
    }

    private MaintenanceRequest request(String externalId) {
        Instant now = Instant.now();
        MaintenanceRequest request = new MaintenanceRequest();
        request.setExternalRequestId(externalId);
        request.setTitle("Test API request");
        request.setDescription("Test request fixture");
        request.setSourceAsset("Gun 1");
        request.setPriority(Priority.NORMAL);
        request.setRemoteStatus("ApprovedForVendor");
        request.setLocalStatus(LocalStatus.NEW);
        request.setRequestedAt(now);
        request.setImportedAt(now);
        request.setRawSnapshotJson("{}");
        return requests.saveAndFlush(request);
    }
}

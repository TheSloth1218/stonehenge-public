package gov.sca.fulfillment.fulfillment;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class FulfillmentOrderStateTest {
    @Test
    void pickingIsEditableButNotShippable() {
        FulfillmentOrder order = order(OrderStatus.PICKING);
        assertThat(order.itemEditable()).isTrue();
        assertThat(order.shippable()).isFalse();
    }

    @Test
    void reviewRemainsEditableAndShippableUntilShipment() {
        FulfillmentOrder order = order(OrderStatus.REVIEW);
        assertThat(order.itemEditable()).isTrue();
        assertThat(order.shippable()).isTrue();
    }

    @Test
    void completedOrderIsTerminalOnly() {
        FulfillmentOrder order = order(OrderStatus.COMPLETION_SENT);
        assertThat(order.itemEditable()).isFalse();
        assertThat(order.shippable()).isFalse();
        assertThat(order.terminal()).isTrue();
    }

    private FulfillmentOrder order(OrderStatus status) {
        FulfillmentOrder order = new FulfillmentOrder();
        order.setStatus(status);
        return order;
    }
}

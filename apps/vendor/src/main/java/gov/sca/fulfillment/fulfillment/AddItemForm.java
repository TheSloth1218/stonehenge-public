package gov.sca.fulfillment.fulfillment;

import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;

public record AddItemForm(@NotNull Long partId, @Min(1) int quantity) {
}

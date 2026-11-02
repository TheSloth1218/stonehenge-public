package gov.sca.fulfillment.security;

import gov.sca.fulfillment.common.AppException;
import gov.sca.fulfillment.fulfillment.FulfillmentOrder;
import gov.sca.fulfillment.users.AppUser;
import gov.sca.fulfillment.users.CurrentUserService;
import org.springframework.security.core.Authentication;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.stereotype.Service;

@Service
public class OrderAuthorizationService {
    private final CurrentUserService currentUserService;

    public OrderAuthorizationService(CurrentUserService currentUserService) {
        this.currentUserService = currentUserService;
    }

    public void requireAssignedEmployeeOrAdmin(FulfillmentOrder order) {
        Authentication authentication = SecurityContextHolder.getContext().getAuthentication();
        boolean admin = authentication != null && authentication.getAuthorities().stream()
                .anyMatch(authority -> "ROLE_ADMIN".equals(authority.getAuthority()));
        if (admin) {
            return;
        }
        AppUser current = currentUserService.currentUser();
        if (order.getAssignedUser() == null || !order.getAssignedUser().getId().equals(current.getId())) {
            throw new AppException("This fulfillment order is assigned to another employee.");
        }
    }
}

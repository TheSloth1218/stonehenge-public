package gov.sca.fulfillment.security;

import org.springframework.security.core.Authentication;
import org.springframework.stereotype.Service;

@Service("authorizationService")
public class AuthorizationService {
    private final TrainingProperties training;

    public AuthorizationService(TrainingProperties training) {
        this.training = training;
    }

    public boolean isAdmin(Authentication authentication) {
        return has(authentication, "ROLE_ADMIN");
    }

    public boolean adminOrTrainingEmployee(Authentication authentication) {
        return isAdmin(authentication) || (training.enabled() && has(authentication, "ROLE_EMPLOYEE"));
    }

    private boolean has(Authentication authentication, String authority) {
        return authentication != null && authentication.getAuthorities().stream()
                .anyMatch(granted -> authority.equals(granted.getAuthority()));
    }
}

package gov.sca.fulfillment.users;

import gov.sca.fulfillment.common.NotFoundException;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.security.core.Authentication;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.stereotype.Service;
import gov.sca.fulfillment.security.SecurityProperties;

@Service
public class CurrentUserService {
    private final AppUserRepository users;
    private final String devUsername;
    private final SecurityProperties security;

    public CurrentUserService(AppUserRepository users, @Value("${app.dev-user}") String devUsername,
                              SecurityProperties security) {
        this.users = users;
        this.devUsername = devUsername;
        this.security = security;
    }

    public AppUser currentUser() {
        Authentication authentication = SecurityContextHolder.getContext().getAuthentication();
        if (authentication != null && authentication.isAuthenticated() && !"anonymousUser".equals(authentication.getName())) {
            return users.findByDexSubject(authentication.getName())
                    .or(() -> users.findByUsername(authentication.getName()))
                    .orElseThrow(() -> new NotFoundException("Authenticated application profile was not found"));
        }
        if (!security.developmentAuth()) {
            throw new NotFoundException("No authenticated application profile is available");
        }
        return users.findByUsername(devUsername)
                .orElseThrow(() -> new NotFoundException("Configured development user was not found: " + devUsername));
    }
}

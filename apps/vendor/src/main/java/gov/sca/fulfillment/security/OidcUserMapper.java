package gov.sca.fulfillment.security;

import gov.sca.fulfillment.users.AppUser;
import gov.sca.fulfillment.users.AppUserRepository;
import gov.sca.fulfillment.users.UserRole;
import org.springframework.security.core.GrantedAuthority;
import org.springframework.security.core.authority.SimpleGrantedAuthority;
import org.springframework.security.oauth2.client.oidc.userinfo.OidcUserRequest;
import org.springframework.security.oauth2.client.oidc.userinfo.OidcUserService;
import org.springframework.security.oauth2.core.oidc.user.DefaultOidcUser;
import org.springframework.security.oauth2.core.oidc.user.OidcUser;
import org.springframework.security.oauth2.client.userinfo.OAuth2UserService;
import org.springframework.security.oauth2.core.OAuth2AuthenticationException;
import org.springframework.security.oauth2.core.OAuth2Error;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.util.ArrayList;
import java.util.Collection;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;

@Service
public class OidcUserMapper implements OAuth2UserService<OidcUserRequest, OidcUser> {
    private static final Logger log = LoggerFactory.getLogger(OidcUserMapper.class);
    private static final String AUTHORIZATION_GROUPS_CLAIM = "groups";
    private final OidcUserService delegate = new OidcUserService();
    private final AppUserRepository users;
    private final SecurityProperties properties;

    public OidcUserMapper(AppUserRepository users, SecurityProperties properties) {
        this.users = users;
        this.properties = properties;
    }

    @Override
    @Transactional
    public OidcUser loadUser(OidcUserRequest request) throws OAuth2AuthenticationException {
        return mapUser(delegate.loadUser(request));
    }

    OidcUser mapUser(OidcUser oidc) throws OAuth2AuthenticationException {
        String subject = oidc.getSubject();
        if (subject == null || subject.isBlank()) {
            throw denied("Dex did not provide an immutable sub claim");
        }

        Set<String> claimedGroups = claimValues(oidc.getClaim(AUTHORIZATION_GROUPS_CLAIM));
        boolean admin = claimedGroups.contains(properties.adminGroup());
        boolean employee = admin || claimedGroups.contains(properties.employeeGroup());
        if (!employee) {
            log.warn("OIDC identity has no recognized Vendor group configuredEmployee={} configuredAdmin={} claimedGroups={}",
                    properties.employeeGroup(), properties.adminGroup(), safeGroups(claimedGroups));
            throw denied("Identity is not assigned to a Stonehenge Vendor role");
        }

        UserRole role = admin ? UserRole.ADMIN : UserRole.EMPLOYEE;
        upsertProfile(oidc, subject, role);

        List<GrantedAuthority> authorities = new ArrayList<>(oidc.getAuthorities());
        authorities.add(new SimpleGrantedAuthority("ROLE_EMPLOYEE"));
        if (admin) {
            authorities.add(new SimpleGrantedAuthority("ROLE_ADMIN"));
        }
        return new DefaultOidcUser(authorities, oidc.getIdToken(), oidc.getUserInfo(), "sub");
    }

    private void upsertProfile(OidcUser oidc, String subject, UserRole role) {
        Instant now = Instant.now();
        AppUser user = users.findByDexSubject(subject).orElseGet(AppUser::new);
        String preferredUsername = firstNonBlank(oidc.getPreferredUsername(), subject);
        String storedUsername = users.findByUsername(preferredUsername)
                .filter(existing -> user.getId() == null || !existing.getId().equals(user.getId()))
                .map(existing -> preferredUsername + "-" + Integer.toUnsignedString(subject.hashCode(), 36))
                .orElse(preferredUsername);
        user.setDexSubject(subject);
        user.setUsername(storedUsername);
        user.setDisplayName(firstNonBlank(oidc.getFullName(), oidc.getPreferredUsername(), subject));
        user.setEmail(firstNonBlank(oidc.getEmail(), "unavailable@invalid"));
        user.setRole(role);
        user.setEnabled(true);
        user.setLastLoginAt(now);
        user.setUpdatedAt(now);
        if (user.getCreatedAt() == null) {
            user.setCreatedAt(now);
        }
        users.save(user);
    }

    private Set<String> claimValues(Object raw) {
        Set<String> values = new LinkedHashSet<>();
        if (raw instanceof Collection<?> collection) {
            collection.stream().map(String::valueOf).forEach(values::add);
        } else if (raw instanceof String value) {
            values.addAll(List.of(value.split("[ ,]+")));
        }
        return values;
    }

    private List<String> safeGroups(Set<String> groups) {
        return groups.stream()
                .limit(10)
                .map(group -> group.replaceAll("[^A-Za-z0-9_.@-]", "_"))
                .map(group -> group.substring(0, Math.min(group.length(), 80)))
                .toList();
    }

    private OAuth2AuthenticationException denied(String description) {
        return new OAuth2AuthenticationException(new OAuth2Error("access_denied", description, null));
    }

    private String firstNonBlank(String... values) {
        for (String value : values) {
            if (value != null && !value.isBlank()) {
                return value;
            }
        }
        return "unknown";
    }
}

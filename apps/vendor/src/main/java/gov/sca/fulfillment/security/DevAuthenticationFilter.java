package gov.sca.fulfillment.security;

import gov.sca.fulfillment.users.AppUser;
import gov.sca.fulfillment.users.AppUserRepository;
import gov.sca.fulfillment.users.UserRole;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.authority.SimpleGrantedAuthority;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.filter.OncePerRequestFilter;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

final class DevAuthenticationFilter extends OncePerRequestFilter {
    private final AppUserRepository users;
    private final String username;

    DevAuthenticationFilter(AppUserRepository users, String username) {
        this.users = users;
        this.username = username;
    }

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
            throws ServletException, IOException {
        if (SecurityContextHolder.getContext().getAuthentication() == null) {
            AppUser user = users.findByUsername(username).orElse(null);
            if (user != null && user.isEnabled()) {
                List<SimpleGrantedAuthority> authorities = new ArrayList<>();
                authorities.add(new SimpleGrantedAuthority("ROLE_EMPLOYEE"));
                if (user.getRole() == UserRole.ADMIN) {
                    authorities.add(new SimpleGrantedAuthority("ROLE_ADMIN"));
                }
                SecurityContextHolder.getContext().setAuthentication(
                        new UsernamePasswordAuthenticationToken(user.getUsername(), "N/A", authorities));
            }
        }
        chain.doFilter(request, response);
    }
}

package gov.sca.fulfillment.sync;

import org.springframework.boot.web.client.RestTemplateBuilder;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.http.HttpEntity;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpMethod;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.client.RestClientException;
import org.springframework.web.client.RestClientResponseException;
import org.springframework.web.client.RestTemplate;
import org.springframework.web.util.UriComponentsBuilder;

import java.net.URI;
import java.time.Duration;
import java.util.List;

@Component
public class StonehengeApiClient {
    private static final String API_KEY_HEADER = "X-Stonehenge-Api-Key";
    private final RestTemplate restTemplate;
    private final StonehengeApiProperties properties;

    @Autowired
    public StonehengeApiClient(RestTemplateBuilder builder, StonehengeApiProperties properties) {
        this.properties = properties;
        Duration connect = properties.connectTimeout() == null ? Duration.ofSeconds(2) : properties.connectTimeout();
        Duration read = properties.readTimeout() == null ? Duration.ofSeconds(5) : properties.readTimeout();
        this.restTemplate = builder.setConnectTimeout(connect).setReadTimeout(read).build();
    }

    StonehengeApiClient(RestTemplate restTemplate, StonehengeApiProperties properties) {
        this.restTemplate = restTemplate;
        this.properties = properties;
    }

    public List<VendorPartRequestDto> getPartRequests() {
        return exchange(uri("vendor", "part-requests"), HttpMethod.GET, new HttpEntity<>(headers()),
                new ParameterizedTypeReference<List<VendorPartRequestDto>>() {}, "load vendor part requests");
    }

    public VendorPartRequestDto getPartRequest(String requestId) {
        return exchange(uri("vendor", "part-requests", requestId), HttpMethod.GET, new HttpEntity<>(headers()),
                new ParameterizedTypeReference<VendorPartRequestDto>() {}, "load vendor part request");
    }

    public VendorAcknowledgeDto acknowledgePartRequest(String requestId) {
        return exchange(uri("vendor", "part-requests", requestId, "acknowledge"), HttpMethod.POST,
                new HttpEntity<>(headers()), new ParameterizedTypeReference<VendorAcknowledgeDto>() {},
                "acknowledge vendor part request");
    }

    public void completePartRequest(String requestId) {
        completePartRequest(requestId, "Fulfilled by SCA Parts Fulfillment Portal", null);
    }

    public VendorCompletionDto completePartRequest(String requestId, String completionNote, String trackingReference) {
        VendorCompleteRequestDto body = new VendorCompleteRequestDto(completionNote, trackingReference);
        try {
            return exchange(uri("vendor", "part-requests", requestId, "complete"), HttpMethod.POST,
                    new HttpEntity<>(body, headers()), new ParameterizedTypeReference<VendorCompletionDto>() {},
                    "complete vendor part request");
        } catch (StonehengeApiException ex) {
            throw new CompletionSyncException(ex.getMessage(), ex);
        }
    }

    public List<VendorCompletionDto> getVendorCompletions() {
        return exchange(uri("vendor", "completions"), HttpMethod.GET, new HttpEntity<>(headers()),
                new ParameterizedTypeReference<List<VendorCompletionDto>>() {}, "load vendor completions");
    }

    public List<VendorGunStatusDto> getGunStatus() {
        return exchange(uri("vendor", "guns", "status"), HttpMethod.GET, new HttpEntity<>(headers()),
                new ParameterizedTypeReference<List<VendorGunStatusDto>>() {}, "load vendor gun status");
    }

    private HttpHeaders headers() {
        HttpHeaders headers = new HttpHeaders();
        headers.set(API_KEY_HEADER, properties.apiKey());
        headers.setAccept(List.of(MediaType.APPLICATION_JSON));
        headers.setContentType(MediaType.APPLICATION_JSON);
        return headers;
    }

    private URI uri(String... segments) {
        UriComponentsBuilder builder = UriComponentsBuilder.fromUriString(properties.baseUrl());
        for (String segment : segments) {
            builder.pathSegment(segment);
        }
        return builder.build().encode().toUri();
    }

    private <T> T exchange(URI uri, HttpMethod method, HttpEntity<?> entity,
                           ParameterizedTypeReference<T> responseType, String operation) {
        try {
            T body = restTemplate.exchange(uri, method, entity, responseType).getBody();
            if (body == null) {
                throw new StonehengeApiException("Stonehenge API returned an empty response while attempting to " + operation + ".");
            }
            return body;
        } catch (StonehengeApiException ex) {
            throw ex;
        } catch (RestClientResponseException ex) {
            throw new StonehengeApiException("Stonehenge API could not " + operation + " (HTTP "
                    + ex.getStatusCode().value() + ").", ex);
        } catch (RestClientException ex) {
            throw new StonehengeApiException("Stonehenge API is unavailable while attempting to " + operation + ".", ex);
        }
    }
}

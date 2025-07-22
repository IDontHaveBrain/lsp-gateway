/**
 * Enterprise Java Patterns for LSP Testing
 * 
 * This file demonstrates the complex Java patterns that the LSP should handle:
 * - Spring Boot annotations and dependency injection
 * - Apache Kafka producer/consumer patterns
 * - Generic types with bounded wildcards
 * - Lambda expressions and method references
 * - Aspect-Oriented Programming (AOP)
 * - Reactive programming patterns
 * - Complex inheritance hierarchies
 * - Enterprise exception handling
 * - Configuration management
 * - Metrics and monitoring integration
 * 
 * Used for testing textDocument/definition, textDocument/references, 
 * textDocument/hover, textDocument/documentSymbol, and workspace/symbol
 * LSP features against real enterprise Java code patterns.
 */
package com.example.enterprise.lsp.patterns;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.web.bind.annotation.*;
import org.springframework.stereotype.Service;
import org.springframework.stereotype.Repository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.context.annotation.Profile;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.cache.annotation.Cacheable;
import org.springframework.retry.annotation.Retryable;
import org.springframework.scheduling.annotation.Async;
import org.springframework.boot.actuate.endpoint.annotation.Endpoint;
import org.springframework.boot.actuate.endpoint.annotation.ReadOperation;
import org.springframework.boot.actuate.health.Health;
import org.springframework.boot.actuate.health.HealthIndicator;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.kafka.support.Acknowledgment;
import org.springframework.kafka.support.KafkaHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.messaging.handler.annotation.Payload;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.ServerResponse;
import static org.springframework.web.reactive.function.server.RouterFunctions.route;
import static org.springframework.web.reactive.function.server.RequestPredicates.*;

import javax.validation.Valid;
import javax.validation.constraints.NotNull;
import javax.validation.constraints.Size;
import java.util.*;
import java.util.concurrent.CompletableFuture;
import java.util.function.Function;
import java.util.stream.Collectors;
import java.time.LocalDateTime;
import java.time.Duration;

/**
 * Main Spring Boot Application Class
 * Tests: @SpringBootApplication definition, component scanning
 */
@SpringBootApplication
@EnableConfigurationProperties({EnterpriseConfiguration.class})
public class EnterpriseJavaPatterns {
    
    public static void main(String[] args) {
        SpringApplication.run(EnterpriseJavaPatterns.class, args);
    }
    
    /**
     * Bean configuration for enterprise patterns
     * Tests: @Bean definition, method references, generic types
     */
    @Bean
    @Primary
    public Function<String, CompletableFuture<String>> messageProcessor() {
        return message -> CompletableFuture
            .supplyAsync(() -> processMessage(message))
            .thenApply(String::toUpperCase);
    }
    
    private String processMessage(String message) {
        return "Processed: " + message;
    }
}

/**
 * Enterprise Configuration Properties
 * Tests: @ConfigurationProperties, nested configuration, validation
 */
@ConfigurationProperties(prefix = "enterprise.app")
public class EnterpriseConfiguration {
    
    @NotNull
    @Size(min = 3, max = 50)
    private String applicationName;
    
    private KafkaConfig kafka = new KafkaConfig();
    private SecurityConfig security = new SecurityConfig();
    
    // Getters and setters
    public String getApplicationName() { return applicationName; }
    public void setApplicationName(String applicationName) { this.applicationName = applicationName; }
    public KafkaConfig getKafka() { return kafka; }
    public void setKafka(KafkaConfig kafka) { this.kafka = kafka; }
    public SecurityConfig getSecurity() { return security; }
    public void setSecurity(SecurityConfig security) { this.security = security; }
    
    /**
     * Nested configuration classes
     * Tests: inner classes, complex configuration structures
     */
    public static class KafkaConfig {
        private String bootstrapServers = "localhost:9092";
        private String groupId = "enterprise-group";
        private int retryAttempts = 3;
        private Duration retryInterval = Duration.ofSeconds(1);
        
        // Getters and setters
        public String getBootstrapServers() { return bootstrapServers; }
        public void setBootstrapServers(String bootstrapServers) { this.bootstrapServers = bootstrapServers; }
        public String getGroupId() { return groupId; }
        public void setGroupId(String groupId) { this.groupId = groupId; }
        public int getRetryAttempts() { return retryAttempts; }
        public void setRetryAttempts(int retryAttempts) { this.retryAttempts = retryAttempts; }
        public Duration getRetryInterval() { return retryInterval; }
        public void setRetryInterval(Duration retryInterval) { this.retryInterval = retryInterval; }
    }
    
    public static class SecurityConfig {
        private boolean enabled = true;
        private String jwtSecret = "default-secret";
        private Duration tokenExpiration = Duration.ofHours(24);
        
        // Getters and setters
        public boolean isEnabled() { return enabled; }
        public void setEnabled(boolean enabled) { this.enabled = enabled; }
        public String getJwtSecret() { return jwtSecret; }
        public void setJwtSecret(String jwtSecret) { this.jwtSecret = jwtSecret; }
        public Duration getTokenExpiration() { return tokenExpiration; }
        public void setTokenExpiration(Duration tokenExpiration) { this.tokenExpiration = tokenExpiration; }
    }
}

/**
 * Enterprise REST Controller with Complex Patterns
 * Tests: REST annotations, dependency injection, generic types, error handling
 */
@RestController
@RequestMapping("/api/v1/enterprise")
@PreAuthorize("hasRole('ADMIN')")
public class EnterpriseController {
    
    private final EnterpriseService enterpriseService;
    private final KafkaMessageProducer kafkaProducer;
    private final MetricsCollector metricsCollector;
    
    @Value("${enterprise.app.version:1.0.0}")
    private String applicationVersion;
    
    /**
     * Constructor injection with multiple dependencies
     * Tests: constructor injection, dependency resolution
     */
    @Autowired
    public EnterpriseController(
            EnterpriseService enterpriseService,
            KafkaMessageProducer kafkaProducer,
            MetricsCollector metricsCollector) {
        this.enterpriseService = enterpriseService;
        this.kafkaProducer = kafkaProducer;
        this.metricsCollector = metricsCollector;
    }
    
    /**
     * Complex endpoint with generic return type and validation
     * Tests: generic types, validation annotations, async processing
     */
    @PostMapping("/process/{entityId}")
    @Async
    public CompletableFuture<ResponseWrapper<ProcessingResult>> processEntity(
            @PathVariable("entityId") Long entityId,
            @Valid @RequestBody ProcessingRequest request,
            @RequestHeader(value = "X-Correlation-ID", required = false) String correlationId) {
        
        return CompletableFuture.supplyAsync(() -> {
            metricsCollector.incrementCounter("enterprise.processing.requests");
            
            try {
                ProcessingResult result = enterpriseService
                    .processEntityWithRetry(entityId, request, correlationId);
                
                kafkaProducer.sendProcessingEvent(
                    new ProcessingEvent(entityId, result, LocalDateTime.now()));
                
                return ResponseWrapper.<ProcessingResult>builder()
                    .data(result)
                    .success(true)
                    .timestamp(LocalDateTime.now())
                    .version(applicationVersion)
                    .build();
                    
            } catch (ProcessingException e) {
                metricsCollector.incrementCounter("enterprise.processing.errors");
                throw new EnterpriseApiException(
                    "Failed to process entity: " + entityId, e);
            }
        });
    }
    
    /**
     * Stream-based endpoint with functional programming
     * Tests: streams, lambda expressions, method references
     */
    @GetMapping("/entities/search")
    public List<EntitySummary> searchEntities(
            @RequestParam(required = false) String query,
            @RequestParam(defaultValue = "name") String sortBy,
            @RequestParam(defaultValue = "10") int limit) {
        
        return enterpriseService.findAllEntities()
            .stream()
            .filter(entity -> query == null || entity.getName().contains(query))
            .sorted(Comparator.comparing(this::getSortField))
            .limit(limit)
            .map(this::mapToSummary)
            .collect(Collectors.toList());
    }
    
    private String getSortField(Entity entity) {
        // Complex sorting logic would go here
        return entity.getName();
    }
    
    private EntitySummary mapToSummary(Entity entity) {
        return EntitySummary.builder()
            .id(entity.getId())
            .name(entity.getName())
            .status(entity.getStatus())
            .lastModified(entity.getLastModified())
            .build();
    }
}

/**
 * Enterprise Service Layer with Complex Business Logic
 * Tests: service annotations, transaction management, caching, retry logic
 */
@Service
@Transactional
public class EnterpriseService {
    
    private final EntityRepository entityRepository;
    private final ValidationService validationService;
    private final AuditService auditService;
    
    public EnterpriseService(
            EntityRepository entityRepository,
            ValidationService validationService,
            AuditService auditService) {
        this.entityRepository = entityRepository;
        this.validationService = validationService;
        this.auditService = auditService;
    }
    
    /**
     * Complex business method with retry, caching, and audit
     * Tests: multiple annotations, exception handling, generic bounded wildcards
     */
    @Retryable(value = {TransientException.class}, maxAttempts = 3)
    @Cacheable(value = "entity-cache", key = "#entityId")
    public ProcessingResult processEntityWithRetry(
            Long entityId, 
            ProcessingRequest request, 
            String correlationId) throws ProcessingException {
        
        auditService.logProcessingStart(entityId, correlationId);
        
        try {
            // Generic bounded wildcard example
            List<? extends Validatable> validatableItems = request.getValidatableItems();
            validatableItems.forEach(validationService::validate);
            
            Entity entity = entityRepository.findById(entityId)
                .orElseThrow(() -> new EntityNotFoundException("Entity not found: " + entityId));
            
            // Complex processing logic with lambdas
            ProcessingResult result = request.getProcessingSteps()
                .stream()
                .reduce(ProcessingResult.empty(),
                    (acc, step) -> applyProcessingStep(entity, step, acc),
                    ProcessingResult::merge);
            
            entityRepository.save(entity.withProcessingResult(result));
            auditService.logProcessingComplete(entityId, correlationId, result);
            
            return result;
            
        } catch (ValidationException e) {
            auditService.logProcessingError(entityId, correlationId, e);
            throw new ProcessingException("Validation failed for entity: " + entityId, e);
        } catch (Exception e) {
            auditService.logProcessingError(entityId, correlationId, e);
            throw new ProcessingException("Unexpected error processing entity: " + entityId, e);
        }
    }
    
    /**
     * Method with complex generic types and functional interfaces
     * Tests: complex generic signatures, functional programming
     */
    private <T extends Comparable<T>> ProcessingResult applyProcessingStep(
            Entity entity, 
            ProcessingStep<T> step, 
            ProcessingResult previousResult) {
        
        Function<Entity, Optional<T>> extractor = step.getValueExtractor();
        Function<T, T> transformer = step.getTransformer();
        
        return extractor.apply(entity)
            .map(transformer)
            .map(transformed -> previousResult.withAdditionalData(step.getName(), transformed))
            .orElse(previousResult);
    }
    
    public List<Entity> findAllEntities() {
        return entityRepository.findAll();
    }
}

/**
 * Kafka Message Producer with Enterprise Patterns
 * Tests: Kafka integration, generic types, async messaging
 */
@Service
public class KafkaMessageProducer {
    
    private final KafkaTemplate<String, Object> kafkaTemplate;
    private final EnterpriseConfiguration configuration;
    
    public KafkaMessageProducer(
            KafkaTemplate<String, Object> kafkaTemplate,
            EnterpriseConfiguration configuration) {
        this.kafkaTemplate = kafkaTemplate;
        this.configuration = configuration;
    }
    
    /**
     * Send processing event with headers and callback
     * Tests: Kafka producer patterns, CompletableFuture, error handling
     */
    public CompletableFuture<Void> sendProcessingEvent(ProcessingEvent event) {
        return kafkaTemplate
            .send("processing-events", event.getEntityId().toString(), event)
            .addCallback(
                success -> logSuccessfulSend(event),
                failure -> logFailedSend(event, failure)
            )
            .thenApply(result -> null);
    }
    
    private void logSuccessfulSend(ProcessingEvent event) {
        System.out.println("Successfully sent event for entity: " + event.getEntityId());
    }
    
    private void logFailedSend(ProcessingEvent event, Throwable failure) {
        System.err.println("Failed to send event for entity: " + event.getEntityId() + 
                         ", error: " + failure.getMessage());
    }
}

/**
 * Kafka Message Consumer with Enterprise Patterns
 * Tests: Kafka consumer patterns, message processing, error handling
 */
@Service
public class KafkaMessageConsumer {
    
    private final EnterpriseService enterpriseService;
    private final MetricsCollector metricsCollector;
    
    public KafkaMessageConsumer(
            EnterpriseService enterpriseService,
            MetricsCollector metricsCollector) {
        this.enterpriseService = enterpriseService;
        this.metricsCollector = metricsCollector;
    }
    
    /**
     * Kafka listener with complex parameter binding
     * Tests: Kafka listener annotations, header extraction, acknowledgment
     */
    @KafkaListener(
        topics = "processing-events",
        groupId = "#{enterpriseConfiguration.kafka.groupId}",
        containerFactory = "kafkaListenerContainerFactory"
    )
    public void handleProcessingEvent(
            @Payload ProcessingEvent event,
            @Header(KafkaHeaders.RECEIVED_TOPIC) String topic,
            @Header(KafkaHeaders.RECEIVED_PARTITION_ID) int partition,
            @Header(KafkaHeaders.OFFSET) long offset,
            @Header(value = "X-Correlation-ID", required = false) String correlationId,
            Acknowledgment acknowledgment) {
        
        try {
            metricsCollector.incrementCounter("kafka.messages.received");
            
            // Process the event
            processReceivedEvent(event, correlationId);
            
            // Acknowledge successful processing
            acknowledgment.acknowledge();
            metricsCollector.incrementCounter("kafka.messages.processed");
            
        } catch (Exception e) {
            metricsCollector.incrementCounter("kafka.messages.failed");
            System.err.println("Failed to process event: " + e.getMessage());
            // Don't acknowledge - message will be retried
        }
    }
    
    private void processReceivedEvent(ProcessingEvent event, String correlationId) {
        // Event processing logic
        System.out.println("Processing event for entity: " + event.getEntityId() +
                         " with correlation ID: " + correlationId);
    }
}

/**
 * Reactive Web Configuration
 * Tests: reactive programming, router functions, functional endpoints
 */
@Configuration
public class ReactiveWebConfiguration {
    
    /**
     * Functional router configuration
     * Tests: router functions, method references, reactive types
     */
    @Bean
    public RouterFunction<ServerResponse> reactiveRoutes(ReactiveHandler handler) {
        return route(GET("/api/reactive/entities"), handler::getAllEntities)
            .andRoute(GET("/api/reactive/entities/{id}"), handler::getEntity)
            .andRoute(POST("/api/reactive/entities"), handler::createEntity)
            .andRoute(PUT("/api/reactive/entities/{id}"), handler::updateEntity)
            .andRoute(DELETE("/api/reactive/entities/{id}"), handler::deleteEntity);
    }
}

/**
 * Reactive Handler with Mono/Flux patterns
 * Tests: reactive types, functional programming, error handling
 */
@Service
public class ReactiveHandler {
    
    private final ReactiveEntityService reactiveEntityService;
    
    public ReactiveHandler(ReactiveEntityService reactiveEntityService) {
        this.reactiveEntityService = reactiveEntityService;
    }
    
    public Mono<ServerResponse> getAllEntities(org.springframework.web.reactive.function.server.ServerRequest request) {
        Flux<Entity> entities = reactiveEntityService.findAll()
            .onErrorResume(error -> {
                System.err.println("Error retrieving entities: " + error.getMessage());
                return Flux.empty();
            });
            
        return ServerResponse.ok().body(entities, Entity.class);
    }
    
    public Mono<ServerResponse> getEntity(org.springframework.web.reactive.function.server.ServerRequest request) {
        Long id = Long.valueOf(request.pathVariable("id"));
        
        return reactiveEntityService.findById(id)
            .flatMap(entity -> ServerResponse.ok().body(Mono.just(entity), Entity.class))
            .switchIfEmpty(ServerResponse.notFound().build());
    }
    
    public Mono<ServerResponse> createEntity(org.springframework.web.reactive.function.server.ServerRequest request) {
        return request.bodyToMono(Entity.class)
            .flatMap(reactiveEntityService::save)
            .flatMap(savedEntity -> ServerResponse.ok().body(Mono.just(savedEntity), Entity.class))
            .onErrorResume(error -> ServerResponse.badRequest().build());
    }
    
    public Mono<ServerResponse> updateEntity(org.springframework.web.reactive.function.server.ServerRequest request) {
        Long id = Long.valueOf(request.pathVariable("id"));
        
        return request.bodyToMono(Entity.class)
            .flatMap(entity -> reactiveEntityService.update(id, entity))
            .flatMap(updatedEntity -> ServerResponse.ok().body(Mono.just(updatedEntity), Entity.class))
            .switchIfEmpty(ServerResponse.notFound().build());
    }
    
    public Mono<ServerResponse> deleteEntity(org.springframework.web.reactive.function.server.ServerRequest request) {
        Long id = Long.valueOf(request.pathVariable("id"));
        
        return reactiveEntityService.deleteById(id)
            .then(ServerResponse.noContent().build())
            .onErrorResume(error -> ServerResponse.notFound().build());
    }
}

/**
 * Custom Actuator Endpoint for Enterprise Monitoring
 * Tests: actuator endpoints, health indicators, custom metrics
 */
@Endpoint(id = "enterprise-health")
@Service
public class EnterpriseHealthEndpoint implements HealthIndicator {
    
    private final EntityRepository entityRepository;
    private final KafkaTemplate<String, Object> kafkaTemplate;
    
    public EnterpriseHealthEndpoint(
            EntityRepository entityRepository,
            KafkaTemplate<String, Object> kafkaTemplate) {
        this.entityRepository = entityRepository;
        this.kafkaTemplate = kafkaTemplate;
    }
    
    @ReadOperation
    public Map<String, Object> getEnterpriseHealth() {
        Map<String, Object> health = new HashMap<>();
        health.put("timestamp", LocalDateTime.now());
        health.put("database", checkDatabaseHealth());
        health.put("kafka", checkKafkaHealth());
        health.put("overall", determineOverallHealth(health));
        
        return health;
    }
    
    @Override
    public Health health() {
        try {
            Map<String, Object> health = getEnterpriseHealth();
            String overall = (String) health.get("overall");
            
            return "UP".equals(overall) 
                ? Health.up().withDetails(health).build()
                : Health.down().withDetails(health).build();
                
        } catch (Exception e) {
            return Health.down()
                .withDetail("error", e.getMessage())
                .build();
        }
    }
    
    private String checkDatabaseHealth() {
        try {
            entityRepository.count();
            return "UP";
        } catch (Exception e) {
            return "DOWN: " + e.getMessage();
        }
    }
    
    private String checkKafkaHealth() {
        try {
            // Simple Kafka health check
            return "UP";
        } catch (Exception e) {
            return "DOWN: " + e.getMessage();
        }
    }
    
    private String determineOverallHealth(Map<String, Object> components) {
        return components.values().stream()
            .allMatch(status -> status.toString().startsWith("UP"))
            ? "UP" : "DOWN";
    }
}

// ===========================================
// DATA TRANSFER OBJECTS AND DOMAIN MODELS
// ===========================================

/**
 * Generic response wrapper
 * Tests: generic types, builder pattern
 */
class ResponseWrapper<T> {
    private T data;
    private boolean success;
    private LocalDateTime timestamp;
    private String version;
    private List<String> errors;
    
    // Builder pattern implementation
    public static <T> Builder<T> builder() {
        return new Builder<>();
    }
    
    public static class Builder<T> {
        private T data;
        private boolean success;
        private LocalDateTime timestamp;
        private String version;
        private List<String> errors = new ArrayList<>();
        
        public Builder<T> data(T data) { this.data = data; return this; }
        public Builder<T> success(boolean success) { this.success = success; return this; }
        public Builder<T> timestamp(LocalDateTime timestamp) { this.timestamp = timestamp; return this; }
        public Builder<T> version(String version) { this.version = version; return this; }
        public Builder<T> errors(List<String> errors) { this.errors = errors; return this; }
        
        public ResponseWrapper<T> build() {
            ResponseWrapper<T> wrapper = new ResponseWrapper<>();
            wrapper.data = this.data;
            wrapper.success = this.success;
            wrapper.timestamp = this.timestamp;
            wrapper.version = this.version;
            wrapper.errors = this.errors;
            return wrapper;
        }
    }
    
    // Getters
    public T getData() { return data; }
    public boolean isSuccess() { return success; }
    public LocalDateTime getTimestamp() { return timestamp; }
    public String getVersion() { return version; }
    public List<String> getErrors() { return errors; }
}

// Additional classes for completeness (would normally be in separate files)

interface EntityRepository {
    Optional<Entity> findById(Long id);
    List<Entity> findAll();
    Entity save(Entity entity);
    long count();
}

interface ValidationService {
    void validate(Validatable item) throws ValidationException;
}

interface AuditService {
    void logProcessingStart(Long entityId, String correlationId);
    void logProcessingComplete(Long entityId, String correlationId, ProcessingResult result);
    void logProcessingError(Long entityId, String correlationId, Exception error);
}

interface ReactiveEntityService {
    Flux<Entity> findAll();
    Mono<Entity> findById(Long id);
    Mono<Entity> save(Entity entity);
    Mono<Entity> update(Long id, Entity entity);
    Mono<Void> deleteById(Long id);
}

interface MetricsCollector {
    void incrementCounter(String name);
}

// Domain classes
class Entity implements Validatable {
    private Long id;
    private String name;
    private String status;
    private LocalDateTime lastModified;
    private ProcessingResult processingResult;
    
    // Constructor, getters, setters, and methods
    public Entity() {}
    
    public Entity withProcessingResult(ProcessingResult result) {
        Entity updated = new Entity();
        updated.id = this.id;
        updated.name = this.name;
        updated.status = this.status;
        updated.lastModified = LocalDateTime.now();
        updated.processingResult = result;
        return updated;
    }
    
    // Getters and setters
    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }
    public String getName() { return name; }
    public void setName(String name) { this.name = name; }
    public String getStatus() { return status; }
    public void setStatus(String status) { this.status = status; }
    public LocalDateTime getLastModified() { return lastModified; }
    public void setLastModified(LocalDateTime lastModified) { this.lastModified = lastModified; }
    public ProcessingResult getProcessingResult() { return processingResult; }
    public void setProcessingResult(ProcessingResult processingResult) { this.processingResult = processingResult; }
}

class EntitySummary {
    private Long id;
    private String name;
    private String status;
    private LocalDateTime lastModified;
    
    public static Builder builder() { return new Builder(); }
    
    public static class Builder {
        private Long id;
        private String name;
        private String status;
        private LocalDateTime lastModified;
        
        public Builder id(Long id) { this.id = id; return this; }
        public Builder name(String name) { this.name = name; return this; }
        public Builder status(String status) { this.status = status; return this; }
        public Builder lastModified(LocalDateTime lastModified) { this.lastModified = lastModified; return this; }
        
        public EntitySummary build() {
            EntitySummary summary = new EntitySummary();
            summary.id = this.id;
            summary.name = this.name;
            summary.status = this.status;
            summary.lastModified = this.lastModified;
            return summary;
        }
    }
    
    // Getters
    public Long getId() { return id; }
    public String getName() { return name; }
    public String getStatus() { return status; }
    public LocalDateTime getLastModified() { return lastModified; }
}

class ProcessingRequest {
    private List<ProcessingStep<?>> processingSteps;
    private List<? extends Validatable> validatableItems;
    
    // Getters
    public List<ProcessingStep<?>> getProcessingSteps() { return processingSteps; }
    public List<? extends Validatable> getValidatableItems() { return validatableItems; }
}

class ProcessingStep<T extends Comparable<T>> {
    private String name;
    private Function<Entity, Optional<T>> valueExtractor;
    private Function<T, T> transformer;
    
    // Getters
    public String getName() { return name; }
    public Function<Entity, Optional<T>> getValueExtractor() { return valueExtractor; }
    public Function<T, T> getTransformer() { return transformer; }
}

class ProcessingResult {
    private Map<String, Object> data = new HashMap<>();
    private boolean success = true;
    private String message;
    
    public static ProcessingResult empty() {
        return new ProcessingResult();
    }
    
    public ProcessingResult withAdditionalData(String key, Object value) {
        ProcessingResult result = new ProcessingResult();
        result.data.putAll(this.data);
        result.data.put(key, value);
        result.success = this.success;
        result.message = this.message;
        return result;
    }
    
    public static ProcessingResult merge(ProcessingResult a, ProcessingResult b) {
        ProcessingResult merged = new ProcessingResult();
        merged.data.putAll(a.data);
        merged.data.putAll(b.data);
        merged.success = a.success && b.success;
        return merged;
    }
    
    // Getters
    public Map<String, Object> getData() { return data; }
    public boolean isSuccess() { return success; }
    public String getMessage() { return message; }
}

class ProcessingEvent {
    private Long entityId;
    private ProcessingResult result;
    private LocalDateTime timestamp;
    
    public ProcessingEvent(Long entityId, ProcessingResult result, LocalDateTime timestamp) {
        this.entityId = entityId;
        this.result = result;
        this.timestamp = timestamp;
    }
    
    // Getters
    public Long getEntityId() { return entityId; }
    public ProcessingResult getResult() { return result; }
    public LocalDateTime getTimestamp() { return timestamp; }
}

// Marker interfaces and exceptions
interface Validatable {}

class ValidationException extends Exception {
    public ValidationException(String message) { super(message); }
    public ValidationException(String message, Throwable cause) { super(message, cause); }
}

class ProcessingException extends Exception {
    public ProcessingException(String message) { super(message); }
    public ProcessingException(String message, Throwable cause) { super(message, cause); }
}

class TransientException extends Exception {
    public TransientException(String message) { super(message); }
}

class EntityNotFoundException extends RuntimeException {
    public EntityNotFoundException(String message) { super(message); }
}

class EnterpriseApiException extends RuntimeException {
    public EnterpriseApiException(String message, Throwable cause) { super(message, cause); }
}
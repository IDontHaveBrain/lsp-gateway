package com.example.lsp.controller;

import com.example.lsp.dto.ProductDto;
import com.example.lsp.dto.ProductRequestDto;
import com.example.lsp.entity.Product;
import com.example.lsp.service.ProductService;
import com.example.lsp.service.UserService;
import com.example.lsp.exception.ProductNotFoundException;
import com.example.lsp.exception.ValidationException;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.web.PageableDefault;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.*;

import javax.validation.Valid;
import javax.validation.constraints.Min;
import javax.validation.constraints.NotBlank;
import javax.validation.constraints.NotNull;
import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.stream.Collectors;

import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.Parameter;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.responses.ApiResponses;
import io.swagger.v3.oas.annotations.tags.Tag;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;

/**
 * Product REST Controller
 * Demonstrates Spring Boot patterns for LSP testing
 */
@RestController
@RequestMapping("/api/v1/products")
@Validated
@RequiredArgsConstructor
@Slf4j
@Tag(name = "Products", description = "Product management operations")
@ConditionalOnProperty(name = "app.features.products", havingValue = "true", matchIfMissing = true)
public class ProductController {

    private final ProductService productService;
    private final UserService userService;
    private final ApplicationEventPublisher eventPublisher;
    
    @Autowired
    @Qualifier("redisProductCache")
    private ProductCache productCache;

    /**
     * Get all products with pagination and filtering
     */
    @GetMapping
    @Operation(summary = "Get all products", description = "Retrieve a paginated list of products with optional filtering")
    @ApiResponses(value = {
        @ApiResponse(responseCode = "200", description = "Products retrieved successfully"),
        @ApiResponse(responseCode = "400", description = "Invalid request parameters")
    })
    public ResponseEntity<Page<ProductDto>> getAllProducts(
            @RequestParam(required = false) 
            @Parameter(description = "Filter by product name") 
            String name,
            
            @RequestParam(required = false) 
            @Parameter(description = "Filter by category ID") 
            Long categoryId,
            
            @RequestParam(required = false) 
            @Parameter(description = "Minimum price filter") 
            @Min(0) BigDecimal minPrice,
            
            @RequestParam(required = false) 
            @Parameter(description = "Maximum price filter") 
            BigDecimal maxPrice,
            
            @RequestParam(defaultValue = "false") 
            @Parameter(description = "Include only active products") 
            boolean activeOnly,
            
            @PageableDefault(size = 20, sort = "createdAt") 
            @Parameter(description = "Pagination parameters") 
            Pageable pageable) {

        log.info("Retrieving products with filters: name={}, categoryId={}, minPrice={}, maxPrice={}, activeOnly={}", 
                name, categoryId, minPrice, maxPrice, activeOnly);

        try {
            ProductFilter filter = ProductFilter.builder()
                    .name(name)
                    .categoryId(categoryId)
                    .minPrice(minPrice)
                    .maxPrice(maxPrice)
                    .activeOnly(activeOnly)
                    .build();

            Page<ProductDto> products = productService.findProductsWithFilter(filter, pageable);
            
            log.info("Retrieved {} products", products.getTotalElements());
            return ResponseEntity.ok(products);
            
        } catch (Exception e) {
            log.error("Error retrieving products", e);
            throw new RuntimeException("Failed to retrieve products", e);
        }
    }

    /**
     * Get product by ID
     */
    @GetMapping("/{id}")
    @Operation(summary = "Get product by ID", description = "Retrieve a single product by its unique identifier")
    @ApiResponses(value = {
        @ApiResponse(responseCode = "200", description = "Product found"),
        @ApiResponse(responseCode = "404", description = "Product not found")
    })
    public ResponseEntity<ProductDto> getProductById(
            @PathVariable 
            @Parameter(description = "Product ID", required = true) 
            @NotNull Long id) {

        log.debug("Retrieving product with ID: {}", id);

        // Check cache first
        Optional<ProductDto> cachedProduct = productCache.get(id);
        if (cachedProduct.isPresent()) {
            log.debug("Product {} found in cache", id);
            return ResponseEntity.ok(cachedProduct.get());
        }

        Optional<ProductDto> product = productService.findProductById(id);
        if (product.isEmpty()) {
            log.warn("Product with ID {} not found", id);
            throw new ProductNotFoundException("Product not found with ID: " + id);
        }

        // Cache the result
        productCache.put(id, product.get());
        
        return ResponseEntity.ok(product.get());
    }

    /**
     * Create new product
     */
    @PostMapping
    @PreAuthorize("hasRole('ADMIN') or hasRole('PRODUCT_MANAGER')")
    @Operation(summary = "Create new product", description = "Create a new product with the provided details")
    @ApiResponses(value = {
        @ApiResponse(responseCode = "201", description = "Product created successfully"),
        @ApiResponse(responseCode = "400", description = "Invalid product data"),
        @ApiResponse(responseCode = "403", description = "Insufficient permissions")
    })
    @Transactional
    public ResponseEntity<ProductDto> createProduct(
            @RequestBody 
            @Valid 
            @Parameter(description = "Product creation request", required = true) 
            ProductRequestDto request,
            
            @AuthenticationPrincipal 
            UserPrincipal currentUser) {

        log.info("Creating new product: {} by user: {}", request.getName(), currentUser.getUsername());

        try {
            // Validate business rules
            validateProductRequest(request);
            
            // Create the product
            ProductDto createdProduct = productService.createProduct(request, currentUser.getId());
            
            // Publish product created event
            ProductCreatedEvent event = new ProductCreatedEvent(this, createdProduct, currentUser);
            eventPublisher.publishEvent(event);
            
            // Clear related caches
            productCache.evictCategoryProducts(request.getCategoryId());
            
            log.info("Product created successfully with ID: {}", createdProduct.getId());
            return ResponseEntity.status(HttpStatus.CREATED).body(createdProduct);
            
        } catch (ValidationException e) {
            log.warn("Product creation validation failed: {}", e.getMessage());
            throw e;
        } catch (Exception e) {
            log.error("Error creating product", e);
            throw new RuntimeException("Failed to create product", e);
        }
    }

    /**
     * Update existing product
     */
    @PutMapping("/{id}")
    @PreAuthorize("hasRole('ADMIN') or (hasRole('PRODUCT_MANAGER') and @productService.isProductOwnedByUser(#id, authentication.principal.id))")
    @Operation(summary = "Update product", description = "Update an existing product with new details")
    @ApiResponses(value = {
        @ApiResponse(responseCode = "200", description = "Product updated successfully"),
        @ApiResponse(responseCode = "400", description = "Invalid product data"),
        @ApiResponse(responseCode = "404", description = "Product not found"),
        @ApiResponse(responseCode = "403", description = "Insufficient permissions")
    })
    @Transactional
    public ResponseEntity<ProductDto> updateProduct(
            @PathVariable 
            @Parameter(description = "Product ID", required = true) 
            @NotNull Long id,
            
            @RequestBody 
            @Valid 
            @Parameter(description = "Product update request", required = true) 
            ProductRequestDto request,
            
            @AuthenticationPrincipal 
            UserPrincipal currentUser) {

        log.info("Updating product ID: {} by user: {}", id, currentUser.getUsername());

        try {
            // Check if product exists
            if (!productService.existsById(id)) {
                throw new ProductNotFoundException("Product not found with ID: " + id);
            }

            // Validate business rules
            validateProductUpdateRequest(id, request);
            
            // Update the product
            ProductDto updatedProduct = productService.updateProduct(id, request, currentUser.getId());
            
            // Publish product updated event
            ProductUpdatedEvent event = new ProductUpdatedEvent(this, updatedProduct, currentUser);
            eventPublisher.publishEvent(event);
            
            // Clear cache
            productCache.evict(id);
            productCache.evictCategoryProducts(request.getCategoryId());
            
            log.info("Product updated successfully: {}", id);
            return ResponseEntity.ok(updatedProduct);
            
        } catch (ProductNotFoundException | ValidationException e) {
            throw e;
        } catch (Exception e) {
            log.error("Error updating product {}", id, e);
            throw new RuntimeException("Failed to update product", e);
        }
    }

    /**
     * Delete product
     */
    @DeleteMapping("/{id}")
    @PreAuthorize("hasRole('ADMIN')")
    @Operation(summary = "Delete product", description = "Delete a product by its ID")
    @ApiResponses(value = {
        @ApiResponse(responseCode = "204", description = "Product deleted successfully"),
        @ApiResponse(responseCode = "404", description = "Product not found"),
        @ApiResponse(responseCode = "403", description = "Insufficient permissions")
    })
    @Transactional
    public ResponseEntity<Void> deleteProduct(
            @PathVariable 
            @Parameter(description = "Product ID", required = true) 
            @NotNull Long id,
            
            @AuthenticationPrincipal 
            UserPrincipal currentUser) {

        log.info("Deleting product ID: {} by user: {}", id, currentUser.getUsername());

        try {
            Optional<ProductDto> existingProduct = productService.findProductById(id);
            if (existingProduct.isEmpty()) {
                throw new ProductNotFoundException("Product not found with ID: " + id);
            }

            // Soft delete the product
            productService.deleteProduct(id, currentUser.getId());
            
            // Publish product deleted event
            ProductDeletedEvent event = new ProductDeletedEvent(this, existingProduct.get(), currentUser);
            eventPublisher.publishEvent(event);
            
            // Clear cache
            productCache.evict(id);
            productCache.evictCategoryProducts(existingProduct.get().getCategoryId());
            
            log.info("Product deleted successfully: {}", id);
            return ResponseEntity.noContent().build();
            
        } catch (ProductNotFoundException e) {
            throw e;
        } catch (Exception e) {
            log.error("Error deleting product {}", id, e);
            throw new RuntimeException("Failed to delete product", e);
        }
    }

    /**
     * Get products by category
     */
    @GetMapping("/category/{categoryId}")
    @Operation(summary = "Get products by category", description = "Retrieve all products in a specific category")
    public ResponseEntity<List<ProductDto>> getProductsByCategory(
            @PathVariable 
            @Parameter(description = "Category ID", required = true) 
            @NotNull Long categoryId,
            
            @RequestParam(defaultValue = "true") 
            @Parameter(description = "Include only active products") 
            boolean activeOnly) {

        log.debug("Retrieving products for category: {}", categoryId);

        try {
            // Check cache first
            Optional<List<ProductDto>> cachedProducts = productCache.getCategoryProducts(categoryId, activeOnly);
            if (cachedProducts.isPresent()) {
                return ResponseEntity.ok(cachedProducts.get());
            }

            List<ProductDto> products = productService.findProductsByCategory(categoryId, activeOnly);
            
            // Cache the results
            productCache.putCategoryProducts(categoryId, activeOnly, products);
            
            return ResponseEntity.ok(products);
            
        } catch (Exception e) {
            log.error("Error retrieving products for category {}", categoryId, e);
            throw new RuntimeException("Failed to retrieve products for category", e);
        }
    }

    /**
     * Search products
     */
    @GetMapping("/search")
    @Operation(summary = "Search products", description = "Search products using full-text search")
    public ResponseEntity<Page<ProductDto>> searchProducts(
            @RequestParam 
            @Parameter(description = "Search query", required = true) 
            @NotBlank String query,
            
            @PageableDefault(size = 20) 
            Pageable pageable) {

        log.info("Searching products with query: {}", query);

        try {
            Page<ProductDto> searchResults = productService.searchProducts(query, pageable);
            return ResponseEntity.ok(searchResults);
            
        } catch (Exception e) {
            log.error("Error searching products with query: {}", query, e);
            throw new RuntimeException("Failed to search products", e);
        }
    }

    /**
     * Get product statistics
     */
    @GetMapping("/stats")
    @PreAuthorize("hasRole('ADMIN') or hasRole('ANALYST')")
    @Operation(summary = "Get product statistics", description = "Retrieve product statistics and analytics")
    public ResponseEntity<ProductStatsDto> getProductStatistics(
            @RequestParam(required = false) 
            @Parameter(description = "Start date for statistics") 
            LocalDateTime startDate,
            
            @RequestParam(required = false) 
            @Parameter(description = "End date for statistics") 
            LocalDateTime endDate) {

        log.info("Retrieving product statistics from {} to {}", startDate, endDate);

        try {
            ProductStatsDto stats = productService.getProductStatistics(startDate, endDate);
            return ResponseEntity.ok(stats);
            
        } catch (Exception e) {
            log.error("Error retrieving product statistics", e);
            throw new RuntimeException("Failed to retrieve product statistics", e);
        }
    }

    /**
     * Bulk update products
     */
    @PatchMapping("/bulk")
    @PreAuthorize("hasRole('ADMIN')")
    @Operation(summary = "Bulk update products", description = "Update multiple products in a single request")
    @Transactional
    public ResponseEntity<List<ProductDto>> bulkUpdateProducts(
            @RequestBody 
            @Valid 
            @Parameter(description = "Bulk update request", required = true) 
            BulkProductUpdateRequest request,
            
            @AuthenticationPrincipal 
            UserPrincipal currentUser) {

        log.info("Bulk updating {} products by user: {}", request.getProductIds().size(), currentUser.getUsername());

        try {
            List<ProductDto> updatedProducts = productService.bulkUpdateProducts(request, currentUser.getId());
            
            // Clear caches for affected products
            request.getProductIds().forEach(productCache::evict);
            
            // Publish bulk update event
            BulkProductUpdateEvent event = new BulkProductUpdateEvent(this, updatedProducts, currentUser);
            eventPublisher.publishEvent(event);
            
            log.info("Bulk update completed for {} products", updatedProducts.size());
            return ResponseEntity.ok(updatedProducts);
            
        } catch (Exception e) {
            log.error("Error during bulk product update", e);
            throw new RuntimeException("Failed to bulk update products", e);
        }
    }

    /**
     * Async product export
     */
    @PostMapping("/export")
    @PreAuthorize("hasRole('ADMIN') or hasRole('EXPORT_USER')")
    @Operation(summary = "Export products", description = "Start an asynchronous product export job")
    public ResponseEntity<ExportJobDto> exportProducts(
            @RequestBody 
            @Valid 
            @Parameter(description = "Export configuration", required = true) 
            ProductExportRequest request,
            
            @AuthenticationPrincipal 
            UserPrincipal currentUser) {

        log.info("Starting product export by user: {}", currentUser.getUsername());

        try {
            CompletableFuture<ExportJobDto> exportJob = productService.exportProductsAsync(request, currentUser.getId());
            
            // Return job ID immediately
            ExportJobDto jobInfo = ExportJobDto.builder()
                    .jobId(UUID.randomUUID().toString())
                    .status(ExportStatus.STARTED)
                    .createdBy(currentUser.getUsername())
                    .createdAt(LocalDateTime.now())
                    .build();
            
            return ResponseEntity.accepted().body(jobInfo);
            
        } catch (Exception e) {
            log.error("Error starting product export", e);
            throw new RuntimeException("Failed to start product export", e);
        }
    }

    // Private helper methods

    private void validateProductRequest(ProductRequestDto request) {
        if (request.getPrice() != null && request.getPrice().compareTo(BigDecimal.ZERO) <= 0) {
            throw new ValidationException("Product price must be greater than zero");
        }

        if (request.getCostPrice() != null && request.getPrice() != null && 
            request.getCostPrice().compareTo(request.getPrice()) > 0) {
            throw new ValidationException("Cost price cannot be greater than selling price");
        }

        // Check if category exists
        if (!userService.categoryExists(request.getCategoryId())) {
            throw new ValidationException("Category does not exist: " + request.getCategoryId());
        }

        // Check for duplicate product name in the same category
        if (productService.existsByNameAndCategory(request.getName(), request.getCategoryId())) {
            throw new ValidationException("Product with this name already exists in the category");
        }
    }

    private void validateProductUpdateRequest(Long productId, ProductRequestDto request) {
        validateProductRequest(request);

        // Additional validation for updates
        if (productService.hasActiveOrders(productId) && !request.isActive()) {
            throw new ValidationException("Cannot deactivate product with active orders");
        }
    }

    /**
     * Exception handler for controller-specific exceptions
     */
    @ExceptionHandler(ProductNotFoundException.class)
    public ResponseEntity<ErrorResponse> handleProductNotFoundException(ProductNotFoundException ex) {
        log.warn("Product not found: {}", ex.getMessage());
        
        ErrorResponse error = ErrorResponse.builder()
                .error("PRODUCT_NOT_FOUND")
                .message(ex.getMessage())
                .timestamp(LocalDateTime.now())
                .build();
        
        return ResponseEntity.status(HttpStatus.NOT_FOUND).body(error);
    }

    @ExceptionHandler(ValidationException.class)
    public ResponseEntity<ErrorResponse> handleValidationException(ValidationException ex) {
        log.warn("Validation error: {}", ex.getMessage());
        
        ErrorResponse error = ErrorResponse.builder()
                .error("VALIDATION_ERROR")
                .message(ex.getMessage())
                .timestamp(LocalDateTime.now())
                .build();
        
        return ResponseEntity.status(HttpStatus.BAD_REQUEST).body(error);
    }

    /**
     * Health check endpoint
     */
    @GetMapping("/health")
    @Operation(summary = "Health check", description = "Check if the product service is healthy")
    public ResponseEntity<HealthCheckResponse> healthCheck() {
        try {
            boolean isDatabaseHealthy = productService.isDatabaseHealthy();
            boolean isCacheHealthy = productCache.isHealthy();
            
            HealthCheckResponse response = HealthCheckResponse.builder()
                    .status(isDatabaseHealthy && isCacheHealthy ? "UP" : "DOWN")
                    .database(isDatabaseHealthy ? "UP" : "DOWN")
                    .cache(isCacheHealthy ? "UP" : "DOWN")
                    .timestamp(LocalDateTime.now())
                    .build();
            
            HttpStatus status = response.getStatus().equals("UP") ? HttpStatus.OK : HttpStatus.SERVICE_UNAVAILABLE;
            return ResponseEntity.status(status).body(response);
            
        } catch (Exception e) {
            log.error("Health check failed", e);
            
            HealthCheckResponse response = HealthCheckResponse.builder()
                    .status("DOWN")
                    .database("UNKNOWN")
                    .cache("UNKNOWN")
                    .timestamp(LocalDateTime.now())
                    .error(e.getMessage())
                    .build();
            
            return ResponseEntity.status(HttpStatus.SERVICE_UNAVAILABLE).body(response);
        }
    }
}

// Supporting classes and interfaces

interface ProductCache {
    Optional<ProductDto> get(Long id);
    void put(Long id, ProductDto product);
    void evict(Long id);
    Optional<List<ProductDto>> getCategoryProducts(Long categoryId, boolean activeOnly);
    void putCategoryProducts(Long categoryId, boolean activeOnly, List<ProductDto> products);
    void evictCategoryProducts(Long categoryId);
    boolean isHealthy();
}

// Builder pattern example
@lombok.Builder
class ProductFilter {
    private final String name;
    private final Long categoryId;
    private final BigDecimal minPrice;
    private final BigDecimal maxPrice;
    private final boolean activeOnly;
    
    // Getters would be generated by Lombok
}

// Event classes for demonstration
abstract class ProductEvent {
    protected final Object source;
    protected final ProductDto product;
    protected final UserPrincipal user;
    protected final LocalDateTime timestamp;
    
    protected ProductEvent(Object source, ProductDto product, UserPrincipal user) {
        this.source = source;
        this.product = product;
        this.user = user;
        this.timestamp = LocalDateTime.now();
    }
}

class ProductCreatedEvent extends ProductEvent {
    public ProductCreatedEvent(Object source, ProductDto product, UserPrincipal user) {
        super(source, product, user);
    }
}

class ProductUpdatedEvent extends ProductEvent {
    public ProductUpdatedEvent(Object source, ProductDto product, UserPrincipal user) {
        super(source, product, user);
    }
}

class ProductDeletedEvent extends ProductEvent {
    public ProductDeletedEvent(Object source, ProductDto product, UserPrincipal user) {
        super(source, product, user);
    }
}

class BulkProductUpdateEvent {
    private final Object source;
    private final List<ProductDto> products;
    private final UserPrincipal user;
    private final LocalDateTime timestamp;
    
    public BulkProductUpdateEvent(Object source, List<ProductDto> products, UserPrincipal user) {
        this.source = source;
        this.products = products;
        this.user = user;
        this.timestamp = LocalDateTime.now();
    }
}
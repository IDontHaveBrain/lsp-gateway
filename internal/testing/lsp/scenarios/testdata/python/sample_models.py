"""
Django model examples demonstrating realistic patterns for LSP testing
"""

from django.db import models
from django.contrib.auth.models import AbstractUser
from django.contrib.contenttypes.models import ContentType
from django.contrib.contenttypes.fields import GenericForeignKey
from django.core.validators import MinValueValidator, MaxValueValidator
from django.utils.translation import gettext_lazy as _
from django.utils import timezone
from django.urls import reverse

import uuid
from typing import Optional, Dict, Any


class TimestampedModel(models.Model):
    """Abstract base model with timestamp fields"""
    created_at = models.DateTimeField(auto_now_add=True, verbose_name=_("Created At"))
    updated_at = models.DateTimeField(auto_now=True, verbose_name=_("Updated At"))
    
    class Meta:
        abstract = True


class User(AbstractUser):
    """Extended user model with additional fields"""
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    bio = models.TextField(max_length=500, blank=True, verbose_name=_("Biography"))
    location = models.CharField(max_length=30, blank=True, verbose_name=_("Location"))
    birth_date = models.DateField(null=True, blank=True, verbose_name=_("Birth Date"))
    avatar = models.ImageField(upload_to='avatars/', null=True, blank=True, verbose_name=_("Avatar"))
    is_verified = models.BooleanField(default=False, verbose_name=_("Verified"))
    
    class Meta:
        verbose_name = _("User")
        verbose_name_plural = _("Users")
        db_table = 'auth_user'
    
    def get_absolute_url(self) -> str:
        return reverse('user-detail', kwargs={'pk': self.pk})
    
    def get_full_display_name(self) -> str:
        """Return full name or username if no full name available"""
        full_name = self.get_full_name()
        return full_name if full_name else self.username
    
    @property
    def age(self) -> Optional[int]:
        """Calculate age from birth date"""
        if self.birth_date:
            today = timezone.now().date()
            return today.year - self.birth_date.year - (
                (today.month, today.day) < (self.birth_date.month, self.birth_date.day)
            )
        return None


class Category(TimestampedModel):
    """Product category model"""
    name = models.CharField(max_length=100, unique=True, verbose_name=_("Name"))
    slug = models.SlugField(max_length=100, unique=True, verbose_name=_("Slug"))
    description = models.TextField(blank=True, verbose_name=_("Description"))
    parent = models.ForeignKey(
        'self', 
        on_delete=models.CASCADE, 
        null=True, 
        blank=True, 
        related_name='children',
        verbose_name=_("Parent Category")
    )
    is_active = models.BooleanField(default=True, verbose_name=_("Active"))
    sort_order = models.PositiveIntegerField(default=0, verbose_name=_("Sort Order"))
    
    class Meta:
        verbose_name = _("Category")
        verbose_name_plural = _("Categories")
        ordering = ['sort_order', 'name']
    
    def __str__(self) -> str:
        return self.name
    
    def get_absolute_url(self) -> str:
        return reverse('category-detail', kwargs={'slug': self.slug})
    
    def get_ancestors(self):
        """Get all ancestor categories"""
        ancestors = []
        parent = self.parent
        while parent:
            ancestors.append(parent)
            parent = parent.parent
        return ancestors[::-1]  # Reverse to get root first
    
    def get_descendants(self):
        """Get all descendant categories"""
        descendants = []
        children = list(self.children.all())
        while children:
            child = children.pop(0)
            descendants.append(child)
            children.extend(child.children.all())
        return descendants


class Product(TimestampedModel):
    """Product model with advanced features"""
    name = models.CharField(max_length=200, verbose_name=_("Name"), db_index=True)
    slug = models.SlugField(max_length=200, unique=True, verbose_name=_("Slug"))
    description = models.TextField(verbose_name=_("Description"))
    short_description = models.CharField(max_length=500, verbose_name=_("Short Description"))
    
    category = models.ForeignKey(
        Category, 
        on_delete=models.SET_NULL, 
        null=True, 
        related_name='products',
        verbose_name=_("Category")
    )
    
    # Pricing
    price = models.DecimalField(
        max_digits=10, 
        decimal_places=2, 
        validators=[MinValueValidator(0.01)],
        verbose_name=_("Price")
    )
    cost_price = models.DecimalField(
        max_digits=10, 
        decimal_places=2, 
        null=True, 
        blank=True,
        validators=[MinValueValidator(0.01)],
        verbose_name=_("Cost Price")
    )
    
    # Inventory
    stock_quantity = models.PositiveIntegerField(default=0, verbose_name=_("Stock Quantity"))
    low_stock_threshold = models.PositiveIntegerField(default=10, verbose_name=_("Low Stock Threshold"))
    
    # Status
    is_active = models.BooleanField(default=True, verbose_name=_("Active"))
    is_featured = models.BooleanField(default=False, verbose_name=_("Featured"))
    is_digital = models.BooleanField(default=False, verbose_name=_("Digital Product"))
    
    # SEO
    meta_title = models.CharField(max_length=60, blank=True, verbose_name=_("Meta Title"))
    meta_description = models.CharField(max_length=160, blank=True, verbose_name=_("Meta Description"))
    
    # Ratings
    rating_average = models.DecimalField(
        max_digits=3, 
        decimal_places=2, 
        default=0.00,
        validators=[MinValueValidator(0.00), MaxValueValidator(5.00)],
        verbose_name=_("Average Rating")
    )
    rating_count = models.PositiveIntegerField(default=0, verbose_name=_("Rating Count"))
    
    class Meta:
        verbose_name = _("Product")
        verbose_name_plural = _("Products")
        ordering = ['-created_at', 'name']
        indexes = [
            models.Index(fields=['name', 'category']),
            models.Index(fields=['is_active', 'is_featured']),
            models.Index(fields=['created_at']),
        ]
    
    def __str__(self) -> str:
        return self.name
    
    def get_absolute_url(self) -> str:
        return reverse('product-detail', kwargs={'slug': self.slug})
    
    @property
    def is_in_stock(self) -> bool:
        """Check if product is in stock"""
        return self.stock_quantity > 0
    
    @property
    def is_low_stock(self) -> bool:
        """Check if product is low on stock"""
        return self.stock_quantity <= self.low_stock_threshold
    
    @property
    def profit_margin(self) -> Optional[float]:
        """Calculate profit margin percentage"""
        if self.cost_price:
            profit = float(self.price - self.cost_price)
            return (profit / float(self.cost_price)) * 100
        return None
    
    def update_rating(self, new_rating: float) -> None:
        """Update product rating with new rating"""
        current_total = self.rating_average * self.rating_count
        new_total = current_total + new_rating
        self.rating_count += 1
        self.rating_average = new_total / self.rating_count
        self.save(update_fields=['rating_average', 'rating_count'])


class ProductImage(models.Model):
    """Product image model"""
    product = models.ForeignKey(
        Product, 
        on_delete=models.CASCADE, 
        related_name='images',
        verbose_name=_("Product")
    )
    image = models.ImageField(upload_to='products/', verbose_name=_("Image"))
    alt_text = models.CharField(max_length=200, verbose_name=_("Alt Text"))
    is_primary = models.BooleanField(default=False, verbose_name=_("Primary Image"))
    sort_order = models.PositiveIntegerField(default=0, verbose_name=_("Sort Order"))
    
    class Meta:
        verbose_name = _("Product Image")
        verbose_name_plural = _("Product Images")
        ordering = ['sort_order']
    
    def __str__(self) -> str:
        return f"{self.product.name} - Image {self.id}"


class Review(TimestampedModel):
    """Product review model"""
    product = models.ForeignKey(
        Product, 
        on_delete=models.CASCADE, 
        related_name='reviews',
        verbose_name=_("Product")
    )
    user = models.ForeignKey(
        User, 
        on_delete=models.CASCADE, 
        related_name='reviews',
        verbose_name=_("User")
    )
    rating = models.PositiveSmallIntegerField(
        validators=[MinValueValidator(1), MaxValueValidator(5)],
        verbose_name=_("Rating")
    )
    title = models.CharField(max_length=100, verbose_name=_("Title"))
    content = models.TextField(verbose_name=_("Content"))
    is_verified_purchase = models.BooleanField(default=False, verbose_name=_("Verified Purchase"))
    helpful_votes = models.PositiveIntegerField(default=0, verbose_name=_("Helpful Votes"))
    
    class Meta:
        verbose_name = _("Review")
        verbose_name_plural = _("Reviews")
        ordering = ['-created_at']
        unique_together = ['product', 'user']
    
    def __str__(self) -> str:
        return f"{self.product.name} - {self.rating} stars by {self.user.username}"
    
    def save(self, *args, **kwargs):
        """Override save to update product rating"""
        is_new = self._state.adding
        super().save(*args, **kwargs)
        
        if is_new:
            self.product.update_rating(float(self.rating))


class Order(TimestampedModel):
    """Order model"""
    
    class OrderStatus(models.TextChoices):
        PENDING = 'pending', _('Pending')
        CONFIRMED = 'confirmed', _('Confirmed')
        PROCESSING = 'processing', _('Processing')
        SHIPPED = 'shipped', _('Shipped')
        DELIVERED = 'delivered', _('Delivered')
        CANCELLED = 'cancelled', _('Cancelled')
        REFUNDED = 'refunded', _('Refunded')
    
    order_number = models.CharField(max_length=20, unique=True, verbose_name=_("Order Number"))
    user = models.ForeignKey(
        User, 
        on_delete=models.CASCADE, 
        related_name='orders',
        verbose_name=_("User")
    )
    status = models.CharField(
        max_length=20, 
        choices=OrderStatus.choices, 
        default=OrderStatus.PENDING,
        verbose_name=_("Status")
    )
    total_amount = models.DecimalField(
        max_digits=10, 
        decimal_places=2,
        verbose_name=_("Total Amount")
    )
    shipping_address = models.TextField(verbose_name=_("Shipping Address"))
    notes = models.TextField(blank=True, verbose_name=_("Notes"))
    
    class Meta:
        verbose_name = _("Order")
        verbose_name_plural = _("Orders")
        ordering = ['-created_at']
    
    def __str__(self) -> str:
        return f"Order {self.order_number}"
    
    def get_absolute_url(self) -> str:
        return reverse('order-detail', kwargs={'order_number': self.order_number})
    
    @property
    def is_completed(self) -> bool:
        """Check if order is completed"""
        return self.status in [self.OrderStatus.DELIVERED, self.OrderStatus.REFUNDED]


class OrderItem(models.Model):
    """Order item model"""
    order = models.ForeignKey(
        Order, 
        on_delete=models.CASCADE, 
        related_name='items',
        verbose_name=_("Order")
    )
    product = models.ForeignKey(
        Product, 
        on_delete=models.CASCADE,
        verbose_name=_("Product")
    )
    quantity = models.PositiveIntegerField(verbose_name=_("Quantity"))
    unit_price = models.DecimalField(max_digits=10, decimal_places=2, verbose_name=_("Unit Price"))
    total_price = models.DecimalField(max_digits=10, decimal_places=2, verbose_name=_("Total Price"))
    
    class Meta:
        verbose_name = _("Order Item")
        verbose_name_plural = _("Order Items")
    
    def __str__(self) -> str:
        return f"{self.product.name} x {self.quantity}"
    
    def save(self, *args, **kwargs):
        """Calculate total price on save"""
        self.total_price = self.quantity * self.unit_price
        super().save(*args, **kwargs)


class ActivityLog(models.Model):
    """Generic activity log using content types"""
    user = models.ForeignKey(
        User, 
        on_delete=models.CASCADE, 
        related_name='activities',
        verbose_name=_("User")
    )
    action = models.CharField(max_length=50, verbose_name=_("Action"))
    timestamp = models.DateTimeField(auto_now_add=True, verbose_name=_("Timestamp"))
    
    # Generic foreign key to any model
    content_type = models.ForeignKey(ContentType, on_delete=models.CASCADE)
    object_id = models.PositiveIntegerField()
    content_object = GenericForeignKey('content_type', 'object_id')
    
    metadata = models.JSONField(default=dict, blank=True, verbose_name=_("Metadata"))
    
    class Meta:
        verbose_name = _("Activity Log")
        verbose_name_plural = _("Activity Logs")
        ordering = ['-timestamp']
        indexes = [
            models.Index(fields=['user', 'timestamp']),
            models.Index(fields=['content_type', 'object_id']),
        ]
    
    def __str__(self) -> str:
        return f"{self.user.username} {self.action} {self.content_object}"


# Manager examples
class ActiveProductManager(models.Manager):
    """Manager for active products only"""
    
    def get_queryset(self):
        return super().get_queryset().filter(is_active=True)
    
    def featured(self):
        """Get featured products"""
        return self.get_queryset().filter(is_featured=True)
    
    def in_stock(self):
        """Get products in stock"""
        return self.get_queryset().filter(stock_quantity__gt=0)
    
    def by_category(self, category):
        """Get products by category"""
        return self.get_queryset().filter(category=category)


# Add custom manager to Product
Product.add_to_class('active_objects', ActiveProductManager())


# Signal handlers (would typically be in signals.py)
from django.db.models.signals import pre_save, post_save
from django.dispatch import receiver
from django.utils.text import slugify


@receiver(pre_save, sender=Product)
def generate_product_slug(sender, instance, **kwargs):
    """Generate slug for product if not provided"""
    if not instance.slug:
        instance.slug = slugify(instance.name)


@receiver(post_save, sender=Order)
def log_order_creation(sender, instance, created, **kwargs):
    """Log order creation activity"""
    if created:
        ActivityLog.objects.create(
            user=instance.user,
            action='created_order',
            content_object=instance,
            metadata={'order_total': str(instance.total_amount)}
        )
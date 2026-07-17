resource "aws_s3_bucket" "assets" {
  bucket = "gophoto-static-assets"
}

resource "aws_s3_bucket_public_access_block" "assets" {
  bucket                  = aws_s3_bucket.assets.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_object" "assets_css" {
  for_each     = fileset("${path.module}/../assets/dist/css", "**")
  bucket       = aws_s3_bucket.assets.id
  key          = "dist/css/${each.value}"
  source       = "${path.module}/../assets/dist/css/${each.value}"
  content_type = "text/css"
  etag         = filemd5("${path.module}/../assets/dist/css/${each.value}")
}

resource "aws_s3_object" "assets_js" {
  for_each     = fileset("${path.module}/../assets/dist/js", "**")
  bucket       = aws_s3_bucket.assets.id
  key          = "dist/js/${each.value}"
  source       = "${path.module}/../assets/dist/js/${each.value}"
  content_type = "application/javascript"
  etag         = filemd5("${path.module}/../assets/dist/js/${each.value}")
}

resource "aws_s3_object" "assets_images" {
  for_each = fileset("${path.module}/../assets/images", "**")
  bucket   = aws_s3_bucket.assets.id
  key      = "images/${each.value}"
  source   = "${path.module}/../assets/images/${each.value}"
  content_type = lookup({
    "jpg"  = "image/jpeg"
    "jpeg" = "image/jpeg"
    "png"  = "image/png"
    "gif"  = "image/gif"
    "svg"  = "image/svg+xml"
    "webp" = "image/webp"
  }, split(".", each.value)[length(split(".", each.value)) - 1], "application/octet-stream")
  etag = filemd5("${path.module}/../assets/images/${each.value}")
}

resource "aws_cloudfront_origin_access_control" "assets" {
  name                              = "assets-oac"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_distribution" "assets" {
  enabled             = true
  default_root_object = "index.html"

  origin {
    domain_name              = aws_s3_bucket.assets.bucket_regional_domain_name
    origin_id                = "s3-assets"
    origin_access_control_id = aws_cloudfront_origin_access_control.assets.id
  }

  default_cache_behavior {
    target_origin_id       = "s3-assets"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    cache_policy_id        = "658327ea-f89d-4fab-a63d-7e88639e58f6" # AWS managed CachingOptimized
    compress               = true
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }
}

resource "aws_s3_bucket_policy" "assets" {
  bucket     = aws_s3_bucket.assets.id
  depends_on = [aws_cloudfront_distribution.assets, aws_s3_bucket_public_access_block.assets]
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "cloudfront.amazonaws.com" }
      Action    = "s3:GetObject"
      Resource  = "${aws_s3_bucket.assets.arn}/*"
      Condition = {
        StringEquals = {
          "AWS:SourceArn" = aws_cloudfront_distribution.assets.arn
        }
      }
    }]
  })
}

output "cdn_distribution_id" {
  description = "CloudFront distribution ID for the static assets CDN"
  value       = aws_cloudfront_distribution.assets.id
}

output "cdn_domain_name" {
  description = "CloudFront domain name for the static assets CDN"
  value       = aws_cloudfront_distribution.assets.domain_name
}

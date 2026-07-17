resource "aws_s3_bucket" "main" {
  bucket = "main-gophoto-bucket"

  tags = {
    Name = "main"
  }
}

resource "aws_s3_bucket_versioning" "main" {
  bucket = aws_s3_bucket.main.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "main" {
  bucket = aws_s3_bucket.main.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "main" {
  bucket = aws_s3_bucket.main.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

data "aws_iam_policy_document" "gophoto" {
  statement {
    effect = "Allow"
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
      "s3:ListBucket"
    ]
    resources = [
      aws_s3_bucket.main.arn,
      "${aws_s3_bucket.main.arn}/*"
    ]
  }
}

resource "aws_iam_policy" "gophoto" {
  name        = "gophoto-s3-policy"
  description = "Allow read and write access to the gophoto S3 bucket"
  policy      = data.aws_iam_policy_document.gophoto.json

  tags = merge(var.tags, {
    Name = "gophoto-s3-policy"
  })
}

data "aws_iam_policy_document" "gophoto_assume_role" {
  statement {
    sid     = "AllowEksAuthToAssumeRoleForPodIdentity"
    effect  = "Allow"
    actions = ["sts:AssumeRole", "sts:TagSession"]

    principals {
      type        = "Service"
      identifiers = ["pods.eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "gophoto" {
  name               = "gophoto-s3-role"
  assume_role_policy = data.aws_iam_policy_document.gophoto_assume_role.json

  tags = merge(var.tags, {
    Name = "gophoto-s3-role"
  })
}

resource "aws_iam_role_policy_attachment" "gophoto" {
  role       = aws_iam_role.gophoto.name
  policy_arn = aws_iam_policy.gophoto.arn
}

resource "aws_eks_pod_identity_association" "example" {
  cluster_name    = aws_eks_cluster.main.name
  namespace       = "default"
  service_account = "gophoto"
  role_arn        = aws_iam_role.gophoto.arn
}

resource "aws_elasticache_subnet_group" "main" {
  name       = "gophoto-redis-subnet-group"
  subnet_ids = aws_subnet.private[*].id

  tags = merge(var.tags, {
    Name = "gophoto-redis-subnet-group"
  })
}

resource "aws_elasticache_cluster" "main" {
  cluster_id           = "gophoto-redis"
  engine               = "redis"
  node_type            = "cache.t3.micro"
  num_cache_nodes      = 1
  parameter_group_name = "default.redis7"
  engine_version       = "7.1"
  port                 = 6379

  subnet_group_name  = aws_elasticache_subnet_group.main.name
  security_group_ids = [aws_security_group.redis.id]

  tags = merge(var.tags, {
    Name = "gophoto-redis"
  })
}

resource "aws_security_group" "redis" {
  name        = "gophoto-redis-sg"
  description = "Allow EKS cluster nodes to access the Redis cluster"
  vpc_id      = aws_eks_cluster.main.vpc_config[0].vpc_id

  ingress {
    description     = "Redis from EKS nodes"
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [aws_eks_cluster.main.vpc_config[0].cluster_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "gophoto-redis-sg"
  })
}

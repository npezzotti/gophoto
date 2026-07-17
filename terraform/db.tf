resource "aws_db_subnet_group" "main" {
  name       = "gophoto-db-subnet-group"
  subnet_ids = aws_subnet.private[*].id

  tags = merge(var.tags, {
    Name = "gophoto-db-subnet-group"
  })
}

resource "aws_db_instance" "main" {
  identifier        = "main"
  engine            = "postgres"
  engine_version    = "15"
  instance_class    = "db.t3.micro"
  allocated_storage = 20
  storage_type      = "gp2"


  db_name             = "gophoto"
  username            = var.db_username
  password_wo         = var.db_password
  password_wo_version = 1

  db_subnet_group_name = aws_db_subnet_group.main.name

  publicly_accessible    = false
  skip_final_snapshot    = true
  deletion_protection    = false
  multi_az               = false
  vpc_security_group_ids = [aws_security_group.db.id]

  tags = merge(var.tags,
    {
      Name = "gophoto-db"
  })
}

resource "aws_security_group" "db" {
  name        = "gophoto-db-sg"
  description = "Allow EKS cluster nodes to access the RDS database"
  vpc_id      = aws_eks_cluster.main.vpc_config[0].vpc_id

  ingress {
    description     = "PostgreSQL from EKS nodes"
    from_port       = 5432
    to_port         = 5432
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
    Name = "gophoto-db-sg"
  })
}

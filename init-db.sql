-- Initialize test database with sample data

CREATE TABLE IF NOT EXISTS customers (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    customer_id INT NOT NULL,
    total DECIMAL(10, 2) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (customer_id) REFERENCES customers(id)
);

CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock INT DEFAULT 0,
    category VARCHAR(100)
);

-- Insert sample customers
INSERT INTO customers (name, email) VALUES
('John Doe', 'john@example.com'),
('Jane Smith', 'jane@example.com'),
('Bob Johnson', 'bob@example.com'),
('Alice Williams', 'alice@example.com'),
('Charlie Brown', 'charlie@example.com');

-- Insert sample products
INSERT INTO products (name, price, stock, category) VALUES
('Laptop', 999.99, 50, 'Electronics'),
('Mouse', 29.99, 200, 'Electronics'),
('Keyboard', 79.99, 150, 'Electronics'),
('Monitor', 299.99, 75, 'Electronics'),
('Desk Chair', 199.99, 30, 'Furniture'),
('Standing Desk', 499.99, 20, 'Furniture'),
('Notebook', 5.99, 500, 'Stationery'),
('Pen Set', 12.99, 300, 'Stationery');

-- Insert sample orders
INSERT INTO orders (customer_id, total, status) VALUES
(1, 1099.98, 'completed'),
(1, 29.99, 'completed'),
(2, 799.97, 'completed'),
(2, 299.99, 'pending'),
(3, 499.99, 'completed'),
(4, 18.98, 'completed'),
(4, 1299.97, 'shipped'),
(5, 29.99, 'pending');

-- Custom ENUMs for billing
DO $$ BEGIN
    CREATE TYPE subscription_status AS ENUM ('active', 'past_due', 'canceled', 'trialing');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE payment_status AS ENUM ('pending', 'completed', 'failed', 'refunded');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- 1. Plans Table
CREATE TABLE IF NOT EXISTS plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    slug VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    price_xof INT NOT NULL DEFAULT 0,
    price_usd NUMERIC(6,2) NOT NULL DEFAULT 0.00,
    max_applications INT NOT NULL DEFAULT 1,
    max_cpus NUMERIC(4,2) NOT NULL DEFAULT 0.5,
    max_memory_mb INT NOT NULL DEFAULT 512,
    custom_domains_allowed BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert Default Plans
INSERT INTO plans (name, slug, description, price_xof, price_usd, max_applications, max_cpus, max_memory_mb, custom_domains_allowed)
VALUES 
('Starter (Free)', 'free', 'Ideal for hobbyists and test deployments', 0, 0.00, 1, 0.5, 512, FALSE),
('Pro Developer', 'pro', 'For production web applications and APIs with custom domains', 15000, 25.00, 5, 2.0, 2048, TRUE),
('Team / Enterprise', 'enterprise', 'High-performance microservices, priority build queues and dedicated compute', 60000, 99.00, 25, 8.0, 8192, TRUE)
ON CONFLICT (slug) DO NOTHING;

-- 2. Subscriptions Table
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
    status subscription_status NOT NULL DEFAULT 'active',
    paydunya_invoice_token VARCHAR(255),
    current_period_start TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    current_period_end TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '30 days',
    canceled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_workspace_active_subscription UNIQUE(workspace_id)
);

-- 3. Payments & Invoices Table (PayDunya Transactions)
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
    paydunya_invoice_token VARCHAR(255) NOT NULL UNIQUE,
    paydunya_receipt_url TEXT,
    amount_xof INT NOT NULL,
    payment_method VARCHAR(50), -- Wave, Orange Money, Free Money, Card
    customer_phone VARCHAR(50),
    status payment_status NOT NULL DEFAULT 'pending',
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_workspace_id ON subscriptions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_payments_workspace_id ON payments(workspace_id);
CREATE INDEX IF NOT EXISTS idx_payments_token ON payments(paydunya_invoice_token);

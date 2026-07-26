import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { Card } from '../Card';

describe('Card', () => {
  it('renders the shared heading contract and flush surface variant', () => {
    const html = renderToStaticMarkup(
      <Card
        title="Title"
        subtitle="Description"
        titleMeta={<span>3 items</span>}
        extra={<button type="button">Action</button>}
        variant="flush"
      >
        Body
      </Card>,
    );

    expect(html).toContain('class="card card-flush"');
    expect(html).toContain('class="card-header"');
    expect(html).toContain('class="keeper-card-heading"');
    expect(html).toContain('class="keeper-card-title-track"');
    expect(html).toContain('class="keeper-card-title"');
    expect(html).toContain('class="keeper-card-title-meta"');
    expect(html).toContain('class="keeper-card-subtitle"');
    expect(html).toContain('class="keeper-card-actions"');
    expect(html).toContain('>Description</div>');
    expect(html).toContain('>Body</div>');
  });

  it('keeps the default surface free of the flush modifier', () => {
    const html = renderToStaticMarkup(<Card>Body</Card>);

    expect(html).toContain('class="card"');
  });
});

import { Button } from './Button';

export const Card = ({ title }: { title: string }) => {
  return (
    <section>
      <h2>{title}</h2>
      <Button label={title} onClick={() => undefined} />
    </section>
  );
};

<script lang="ts">
  import Counter from '../lib/Counter.svelte';
  import { addLine, totalQty } from '../lib/cart';

  let lines = addLine([], 'sku-1');

  function reset() {
    lines = [];
    totalQty(lines);
  }
</script>

<h1>Cart</h1>
<Counter start={lines.length} />
<button on:click={reset}>Reset</button>

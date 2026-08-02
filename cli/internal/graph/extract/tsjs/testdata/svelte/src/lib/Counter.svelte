<script lang="ts">
  export let start = 0;

  let count = start;
</script>

<button on:click={() => (count += 1)}>{count}</button>

<style>
  button {
    padding: 4px;
  }
</style>

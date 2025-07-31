<script>
    import ApiClient from "@/utils/ApiClient.js";
    import CommonHelper from "@/utils/CommonHelper.js";
    import { pageTitle } from "@/stores/app.js";
    import { confirm } from "@/stores/confirmation.js";
    import { addSuccessToast } from "@/stores/toasts.js";
    import PageWrapper from "@/components/base/PageWrapper.svelte";
    import Searchbar from "@/components/base/Searchbar.svelte";
    import RefreshButton from "@/components/base/RefreshButton.svelte";
    import Field from "@/components/base/Field.svelte";
    import SortHeader from "@/components/base/SortHeader.svelte";
    import FormattedDate from "@/components/base/FormattedDate.svelte";
    import Scroller from "@/components/base/Scroller.svelte";
    import FunctionUpsertPanel from "@/components/functions/FunctionUpsertPanel.svelte";
    import FunctionInvokePanel from "@/components/functions/FunctionInvokePanel.svelte";

    const limit = 30;

    let functionsPanel;
    let invokePanel;
    let sort = "-created";
    let filter = "";
    let currentPage = 1;
    let functions = [];
    let totalItems = 0;
    let isLoading = false;
    let bulkSelected = {};

    $pageTitle = "PostgreSQL Functions";

    $: canLoadMore = totalItems > functions.length;

    $: if (sort !== null || filter !== null) {
        clearList();
        load();
    }

    export function load() {
        isLoading = true;

        const searchParams = new URLSearchParams({
            page: currentPage,
            perPage: limit,
            sort: sort,
            filter: filter,
        }).toString();

        return ApiClient.send(`/api/pg-functions?${searchParams}`, {
            requestKey: "functions_list",
        })
            .then((result) => {
                if (currentPage > 1) {
                    functions = functions.concat(result.items || []);
                } else {
                    functions = result.items || [];
                }
                totalItems = result.totalItems || 0;
                currentPage = result.page || 1;
            })
            .catch((err) => {
                if (!err?.isAbort) {
                    console.warn(err);
                    clearList();
                    ApiClient.error(err, false, "Failed to load functions.");
                }
            })
            .finally(() => {
                isLoading = false;
            });
    }

    function clearList() {
        functions = [];
        currentPage = 1;
        totalItems = 0;
        bulkSelected = {};
    }

    function loadMore() {
        if (!canLoadMore || isLoading) {
            return;
        }

        currentPage++;
        load();
    }

    function deleteConfirm(functionItem) {
        const name = functionItem.name || "function";

        confirm(`Do you really want to delete function "${name}"?`, () => {
            return deleteFunction(functionItem)
                .then(() => {
                    addSuccessToast(`Successfully deleted function "${name}".`);
                })
                .catch((err) => {
                    ApiClient.error(err);
                });
        });
    }

    function deleteFunction(functionItem) {
        return ApiClient.send(`/api/pg-functions/${functionItem.id}`, {
            method: "DELETE",
        }).then(() => {
            // optimistically remove the deleted function from the list
            CommonHelper.removeByKey(functions, "id", functionItem.id);
            functions = functions;
            totalItems--;

            delete bulkSelected[functionItem.id];
            bulkSelected = bulkSelected;
        });
    }

    function createFunction() {
        functionsPanel?.show();
    }

    function editFunction(functionItem) {
        functionsPanel?.show(functionItem);
    }

    function duplicateFunction(functionItem) {
        const copy = structuredClone(functionItem);
        copy.id = "";
        copy.name = copy.name + "_copy";
        functionsPanel?.show(copy);
    }

    function testFunction(functionItem) {
        invokePanel?.show(functionItem);
    }

    // trigger load on component initialization
    load();
</script>

<PageWrapper>
    <header class="page-header">
        <nav class="breadcrumbs">
            <div class="breadcrumb-item">PostgreSQL Functions</div>
        </nav>
        <RefreshButton on:refresh={() => load()} />
    </header>

    <Searchbar
        value={filter}
        autocompleteCollection={null}
        placeholder={"Search functions..."}
        on:submit={(e) => {
            filter = e.detail;
        }}
    />

    <div class="clearfix m-b-sm">
        <div class="flex-fill" />
        <div class="inline-flex gap-5">
            <button type="button" class="btn btn-sm" on:click={() => createFunction()}>
                <i class="ri-add-line" />
                <span class="txt">New function</span>
            </button>
        </div>
    </div>

    <Scroller class="table-wrapper">
        <table class="table" class:table-loading={isLoading}>
            <thead>
                <tr>
                    <SortHeader class="col-type-text col-field-name" name="name" bind:sort>Name</SortHeader>
                    <SortHeader class="col-type-text col-field-schema" name="schema" bind:sort>
                        Schema
                    </SortHeader>
                    <th class="col-type-text col-field-return-type">Return Type</th>
                    <th class="col-type-text col-field-language">Language</th>
                    <SortHeader class="col-type-date col-field-created" name="created" bind:sort>
                        Created
                    </SortHeader>
                    <SortHeader class="col-type-date col-field-updated" name="updated" bind:sort>
                        Updated
                    </SortHeader>
                    <th class="col-type-action min-width" />
                </tr>
            </thead>
            <tbody>
                {#each functions as functionItem (functionItem.id)}
                    <tr tabindex="0" class="row-handle">
                        <td class="col-type-text col-field-name">
                            <span class="txt">{functionItem.name}</span>
                            {#if !functionItem.isActive}
                                <i
                                    class="ri-error-warning-line txt-sm txt-warning m-l-5"
                                    title="Function is inactive"
                                />
                            {/if}
                        </td>
                        <td class="col-type-text col-field-schema">
                            <span class="txt">{functionItem.schema}</span>
                        </td>
                        <td class="col-type-text col-field-return-type">
                            <span class="txt">{functionItem.returnType}</span>
                        </td>
                        <td class="col-type-text col-field-language">
                            <span class="txt">{functionItem.language}</span>
                        </td>
                        <td class="col-type-date col-field-created">
                            <FormattedDate date={functionItem.created} />
                        </td>
                        <td class="col-type-date col-field-updated">
                            <FormattedDate date={functionItem.updated} />
                        </td>
                        <td class="col-type-action min-width">
                            <div class="inline-flex">
                                <button
                                    type="button"
                                    class="btn btn-sm btn-circle btn-secondary"
                                    title="Test function"
                                    disabled={!functionItem.isActive}
                                    on:click={() => testFunction(functionItem)}
                                >
                                    <i class="ri-play-line" />
                                </button>
                                <button
                                    type="button"
                                    class="btn btn-sm btn-circle btn-secondary"
                                    title="Edit"
                                    on:click={() => editFunction(functionItem)}
                                >
                                    <i class="ri-pencil-line" />
                                </button>
                                <button
                                    type="button"
                                    class="btn btn-sm btn-circle btn-secondary"
                                    title="Duplicate"
                                    on:click={() => duplicateFunction(functionItem)}
                                >
                                    <i class="ri-file-copy-line" />
                                </button>
                                <button
                                    type="button"
                                    class="btn btn-sm btn-circle btn-secondary"
                                    title="Delete"
                                    on:click={() => deleteConfirm(functionItem)}
                                >
                                    <i class="ri-delete-bin-line" />
                                </button>
                            </div>
                        </td>
                    </tr>
                {:else}
                    <tr>
                        <td colspan="7" class="txt-center txt-hint p-xs">
                            <h6>No functions found.</h6>
                            {#if filter?.length}
                                <p>Try changing or removing the search filter.</p>
                            {:else}
                                <p>Create your first PostgreSQL function to get started.</p>
                            {/if}
                        </td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </Scroller>

    {#if isLoading}
        <div class="block txt-center">
            <span class="loader loader-sm" />
        </div>
    {/if}

    {#if canLoadMore}
        <div class="block txt-center m-t-sm">
            <button
                type="button"
                class="btn btn-sm btn-hint"
                disabled={isLoading}
                on:click={() => loadMore()}
            >
                <span class="txt">Load more</span>
            </button>
        </div>
    {/if}
</PageWrapper>

<FunctionUpsertPanel bind:this={functionsPanel} on:save={() => load()} />
<FunctionInvokePanel bind:this={invokePanel} />
